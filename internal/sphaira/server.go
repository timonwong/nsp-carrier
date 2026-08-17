package sphaira

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrProtocol           = errors.New("Sphaira SPH0 protocol error")
	ErrUnsupportedCommand = errors.New("unsupported SPH0 command")
	ErrInvalidRequest     = errors.New("invalid SPH0 request")
	ErrFilenameListLarge  = errors.New("SPH0 filename list exceeds uint32")
)

type RangeReporter interface {
	Requested(sourceID string, offset uint64, size uint64)
	Served(sourceID string, offset uint64, size uint32) error
	Closed(sourceID string)
}

type discardReporter struct{}

func (discardReporter) Requested(string, uint64, uint64)    {}
func (discardReporter) Served(string, uint64, uint32) error { return nil }
func (discardReporter) Closed(string)                       {}

type Server struct {
	catalog  *files.Catalog
	entries  []files.Entry
	list     []byte
	reporter RangeReporter
	trace    protocoltrace.Reporter
}

func NewServer(catalog *files.Catalog, reporter RangeReporter) (*Server, error) {
	return NewServerWithTrace(catalog, reporter, nil)
}

func NewServerWithTrace(catalog *files.Catalog, reporter RangeReporter, trace protocoltrace.Reporter) (*Server, error) {
	if catalog == nil {
		return nil, errors.New("SPH0 server requires a frozen catalog")
	}
	if reporter == nil {
		reporter = discardReporter{}
	}
	entries := catalog.Entries()
	if uint64(len(entries)) > math.MaxUint32 {
		return nil, ErrFilenameListLarge
	}
	var list strings.Builder
	var listSize uint64
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if err := ValidateWireName(entry.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrProtocol, err)
		}
		if previous, exists := seen[entry.Name]; exists {
			return nil, &files.DuplicateBasenameError{Name: entry.Name, Paths: []string{previous, entry.Path}}
		}
		seen[entry.Name] = entry.Path
		if ValidateSourceSize(entry.Size) != nil {
			return nil, fmt.Errorf("%w: %s", ErrFileTooLarge, entry.ID)
		}
		listSize += uint64(len(entry.Name)) + 1
		if listSize > math.MaxUint32 {
			return nil, ErrFilenameListLarge
		}
		list.WriteString(entry.Name)
		list.WriteByte('\n')
	}
	return &Server{
		catalog: catalog, entries: entries, list: []byte(list.String()), reporter: reporter, trace: trace,
	}, nil
}

func (s *Server) Serve(ctx context.Context, link transport.Duplex) error {
	handshake, err := s.readPacket(ctx, link, "handshake")
	if err != nil {
		return err
	}
	s.reportInbound("handshake", handshake, protocoltrace.Record{})
	if handshake.Command() != CommandQuit || handshake.Arg3 != 0 || handshake.Arg4 != 0 || handshake.Arg5 != 0 {
		return s.semanticError(ctx, link, fmt.Errorf("%w: invalid opening command", ErrInvalidRequest))
	}
	if err := s.writeResult(ctx, link, "filename_list", ResultOK, uint32(len(s.list)), 0, protocoltrace.Record{PayloadBytes: uint64(len(s.list))}); err != nil {
		return err
	}
	if err := transport.WriteFull(ctx, link, s.list); err != nil {
		return err
	}

	for {
		packet, err := s.readPacket(ctx, link, "command")
		if err != nil {
			return err
		}
		switch packet.Command() {
		case CommandQuit:
			s.reportInbound("quit", packet, protocoltrace.Record{})
			if packet.Arg3 != 0 || packet.Arg4 != 0 || packet.Arg5 != 0 {
				return s.semanticError(ctx, link, fmt.Errorf("%w: invalid quit command", ErrInvalidRequest))
			}
			if err := s.writeResult(ctx, link, "quit_ack", ResultOK, 0, 0, protocoltrace.Record{}); err != nil {
				return err
			}
			return nil
		case CommandOpen:
			s.reportInbound("open", packet, protocoltrace.Record{Index: packet.Arg3})
			if packet.Arg4 != 0 || packet.Arg5 != 0 || uint64(packet.Arg3) >= uint64(len(s.entries)) {
				return s.semanticError(ctx, link, fmt.Errorf("%w: file index %d", ErrInvalidRequest, packet.Arg3))
			}
			if err := s.serveFile(ctx, link, packet.Arg3); err != nil {
				return err
			}
		default:
			s.reportInbound("command", packet, protocoltrace.Record{})
			return s.semanticError(ctx, link, fmt.Errorf("%w: %d", ErrUnsupportedCommand, packet.Command()))
		}
	}
}

func (s *Server) serveFile(ctx context.Context, link transport.Duplex, index uint32) error {
	entry := s.entries[index]
	arg3, arg4, err := PackFileInfo(uint64(entry.Size), FlagNone)
	if err != nil {
		return s.semanticError(ctx, link, err)
	}
	if err := s.writeResult(ctx, link, "open_result", ResultOK, arg3, arg4, protocoltrace.Record{
		SourceID: entry.ID, Index: index, Size: uint64(entry.Size),
	}); err != nil {
		return err
	}

	for {
		packet, err := s.readPacket(ctx, link, "range")
		if err != nil {
			return err
		}
		offset, requested := packet.Offset(), packet.Size()
		if offset == 0 && requested == 0 && packet.Arg5 == 0 {
			s.reportInbound("close", packet, protocoltrace.Record{SourceID: entry.ID, Index: index})
			if err := s.writeResult(ctx, link, "close_ack", ResultOK, 0, 0, protocoltrace.Record{SourceID: entry.ID, Index: index}); err != nil {
				return err
			}
			s.reporter.Closed(entry.ID)
			return nil
		}
		s.reportInbound("range", packet, protocoltrace.Record{
			SourceID: entry.ID, Index: index, Offset: offset, Size: uint64(requested),
		})
		if packet.Arg5 != 0 || requested > MaxRangeSize || offset > uint64(entry.Size) || offset+uint64(requested) < offset {
			return s.semanticError(ctx, link, fmt.Errorf("%w: offset=%d size=%d", ErrInvalidRequest, offset, requested))
		}

		s.reporter.Requested(entry.ID, offset, uint64(requested))
		reader, available, err := s.catalog.OpenRange(entry.ID, offset, uint64(requested))
		if err != nil {
			return s.semanticError(ctx, link, fmt.Errorf("open source range: %w", err))
		}
		payload := make([]byte, int(available))
		_, readErr := io.ReadFull(reader, payload)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			return s.semanticError(ctx, link, fmt.Errorf("read frozen source: %w", errors.Join(files.ErrSourceChanged, readErr, closeErr)))
		}
		if err := s.writeResult(ctx, link, "range_result", ResultOK, uint32(available), PayloadCRC32C(payload), protocoltrace.Record{
			SourceID: entry.ID, Index: index, Offset: offset, Size: available, PayloadBytes: available,
			IntegrityChecked: true, IntegrityValid: true,
		}); err != nil {
			return err
		}
		written, err := transport.WriteFullCount(ctx, link, payload)
		if written > 0 {
			if progressErr := s.reporter.Served(entry.ID, offset, uint32(written)); progressErr != nil {
				return progressErr
			}
		}
		if err != nil {
			return err
		}
	}
}

func (s *Server) readPacket(ctx context.Context, link transport.Duplex, operation string) (Packet, error) {
	var encoded [PacketSize]byte
	if err := transport.ReadFull(ctx, link, encoded[:]); err != nil {
		return Packet{}, err
	}
	packet, err := DecodePacket(encoded[:])
	if err != nil {
		integrityValid := !errors.Is(err, ErrInvalidHeaderCRC)
		s.report(protocoltrace.Record{
			Direction: protocoltrace.Inbound, Operation: operation,
			IntegrityChecked: true, IntegrityValid: integrityValid,
		})
		return Packet{}, fmt.Errorf("%w: %w", ErrProtocol, err)
	}
	return packet, nil
}

func (s *Server) reportInbound(operation string, packet Packet, record protocoltrace.Record) {
	record.Direction = protocoltrace.Inbound
	record.Operation = operation
	record.Command = packet.Arg2
	record.IntegrityChecked = true
	record.IntegrityValid = true
	s.report(record)
}

func (s *Server) semanticError(ctx context.Context, link transport.Duplex, cause error) error {
	responseErr := s.writeResult(ctx, link, "error_result", ResultError, 0, 0, protocoltrace.Record{})
	return errors.Join(fmt.Errorf("%w: %w", ErrProtocol, cause), responseErr)
}

func (s *Server) writeResult(
	ctx context.Context,
	link transport.Duplex,
	operation string,
	result Result,
	arg3, arg4 uint32,
	record protocoltrace.Record,
) error {
	encoded := EncodeResult(result, arg3, arg4)
	if err := transport.WriteFull(ctx, link, encoded[:]); err != nil {
		return err
	}
	record.Direction = protocoltrace.Outbound
	record.Operation = operation
	record.ResultCode = uint32(result)
	record.HasResult = true
	record.IntegrityChecked = true
	record.IntegrityValid = true
	s.report(record)
	return nil
}

func (s *Server) report(record protocoltrace.Record) {
	if s.trace != nil {
		s.trace.Report(record)
	}
}
