package awoo

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrProtocol           = errors.New("Awoo USB protocol error")
	ErrUnsupportedCommand = errors.New("unsupported Awoo USB command")
)

type RangeReporter interface {
	Requested(sourceID string, offset uint64, size uint64)
	Served(sourceID string, offset uint64, size uint32) error
}

type discardReporter struct{}

func (discardReporter) Requested(string, uint64, uint64)    {}
func (discardReporter) Served(string, uint64, uint32) error { return nil }

type Server struct {
	catalog      *files.Catalog
	sourceByName map[string]string
	reporter     RangeReporter
	listHeader   [16]byte
	listPayload  []byte
	trace        protocoltrace.Reporter
}

func NewServer(catalog *files.Catalog, reporter RangeReporter) (*Server, error) {
	return NewServerWithTrace(catalog, reporter, nil)
}

func NewServerWithTrace(catalog *files.Catalog, reporter RangeReporter, trace protocoltrace.Reporter) (*Server, error) {
	if reporter == nil {
		reporter = discardReporter{}
	}
	server := &Server{
		catalog: catalog, sourceByName: make(map[string]string), reporter: reporter, trace: trace,
	}
	pathByName := make(map[string]string)
	names := make([]string, 0, len(catalog.Entries()))
	for _, entry := range catalog.Entries() {
		if _, exists := server.sourceByName[entry.Name]; exists {
			return nil, &files.DuplicateBasenameError{
				Name: entry.Name, Paths: []string{pathByName[entry.Name], entry.Path},
			}
		}
		server.sourceByName[entry.Name] = entry.ID
		pathByName[entry.Name] = entry.Path
		names = append(names, entry.Name)
	}
	var err error
	server.listHeader, server.listPayload, err = EncodeFileList(names)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return server, nil
}

func (s *Server) Serve(ctx context.Context, link transport.Duplex) error {
	if err := s.sendList(ctx, link); err != nil {
		return err
	}
	for {
		header, err := readCommandHeader(ctx, link)
		if err != nil {
			return err
		}
		if header.Type != CommandTypeRequest {
			return fmt.Errorf("%w: command type %d", ErrProtocol, header.Type)
		}
		switch header.Command {
		case CommandExit:
			if header.DataSize != 0 {
				return fmt.Errorf("%w: exit payload size %d", ErrProtocol, header.DataSize)
			}
			s.report(protocoltrace.Record{
				Direction: protocoltrace.Inbound, Operation: "exit", Command: uint32(header.Command),
			})
			return nil
		case CommandFileRange, CommandFileRangeAlternative:
			if err := s.serveRange(ctx, link, header.Command, header.DataSize); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %d", ErrUnsupportedCommand, header.Command)
		}
	}
}

func (s *Server) sendList(ctx context.Context, link transport.Duplex) error {
	if err := transport.WriteFull(ctx, link, s.listHeader[:]); err != nil {
		return err
	}
	if err := transport.WriteFull(ctx, link, s.listPayload); err != nil {
		return err
	}
	s.report(protocoltrace.Record{
		Direction: protocoltrace.Outbound, Operation: "file_list", PayloadBytes: uint64(len(s.listPayload)),
	})
	return nil
}

func (s *Server) serveRange(ctx context.Context, link transport.Duplex, command CommandID, payloadSize uint64) error {
	if payloadSize < RangeMetadataSize || payloadSize > MaxCommandDataSize {
		return fmt.Errorf("%w: range payload size %d", ErrProtocol, payloadSize)
	}
	payload := make([]byte, int(payloadSize))
	if err := transport.ReadFull(ctx, link, payload); err != nil {
		return err
	}
	request, err := ParseRangeRequest(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	sourceID, ok := s.sourceByName[request.Name]
	if !ok {
		return fmt.Errorf("open range %q: %w", request.Name, files.ErrFileNotFound)
	}
	reader, available, err := s.catalog.OpenRange(sourceID, request.Offset, request.Size)
	if err != nil {
		return fmt.Errorf("open range %q offset=%d size=%d: %w", request.Name, request.Offset, request.Size, err)
	}
	defer reader.Close()
	if available != request.Size {
		return fmt.Errorf("open range %q offset=%d size=%d: %w", request.Name, request.Offset, request.Size, files.ErrRangeOutOfBounds)
	}
	operation := "file_range"
	if command == CommandFileRangeAlternative {
		operation = "file_range_alternative"
	}
	s.report(protocoltrace.Record{
		Direction: protocoltrace.Inbound, Operation: operation, Command: uint32(command),
		PayloadBytes: payloadSize, SourceID: sourceID, Offset: request.Offset, Size: request.Size,
	})
	s.reporter.Requested(sourceID, request.Offset, request.Size)
	response := EncodeCommandHeader(CommandHeader{
		Type: CommandTypeResponse, Command: CommandFileRange, DataSize: request.Size,
	})
	if err := transport.WriteFull(ctx, link, response[:]); err != nil {
		return err
	}
	const chunkSize = 1 << 20
	buffer := make([]byte, min(request.Size, chunkSize))
	remaining := request.Size
	offset := request.Offset
	for remaining > 0 {
		current := min(remaining, uint64(len(buffer)))
		chunk := buffer[:int(current)]
		if _, err := io.ReadFull(reader, chunk); err != nil {
			return fmt.Errorf("%w: %v", files.ErrSourceChanged, err)
		}
		written, err := transport.WriteFullCount(ctx, link, chunk)
		if written > 0 {
			if progressErr := s.reporter.Served(sourceID, offset, uint32(written)); progressErr != nil {
				return progressErr
			}
			offset += uint64(written)
			remaining -= uint64(written)
		}
		if err != nil {
			return err
		}
	}
	s.report(protocoltrace.Record{
		Direction: protocoltrace.Outbound, Operation: "file_range_response", Command: uint32(CommandFileRange),
		PayloadBytes: request.Size, SourceID: sourceID, Offset: request.Offset, Size: request.Size,
	})
	return nil
}

func (s *Server) report(record protocoltrace.Record) {
	if s.trace != nil {
		s.trace.Report(record)
	}
}

func readCommandHeader(ctx context.Context, link transport.Duplex) (CommandHeader, error) {
	encoded := make([]byte, CommandHeaderSize)
	if err := transport.ReadFull(ctx, link, encoded); err != nil {
		return CommandHeader{}, err
	}
	header, err := DecodeCommandHeader(encoded)
	if err != nil {
		return CommandHeader{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return header, nil
}
