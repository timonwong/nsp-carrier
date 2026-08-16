package dbi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrProtocol           = errors.New("DBI0 protocol error")
	ErrUnsupportedCommand = errors.New("unsupported DBI0 command")
)

type Server struct {
	catalog      *files.Catalog
	sourceByName map[string]string
	reporter     RangeReporter
}

type RangeReporter interface {
	Requested(sourceID string, offset uint64, size uint64)
	Served(sourceID string, offset uint64, size uint32) error
}

type discardReporter struct{}

func (discardReporter) Requested(string, uint64, uint64)    {}
func (discardReporter) Served(string, uint64, uint32) error { return nil }

func NewServer(catalog *files.Catalog, reporter RangeReporter) (*Server, error) {
	sourceByName := make(map[string]string)
	pathByName := make(map[string]string)
	if reporter == nil {
		reporter = discardReporter{}
	}
	server := &Server{catalog: catalog, sourceByName: sourceByName, reporter: reporter}
	for _, entry := range catalog.Entries() {
		if _, exists := sourceByName[entry.Name]; exists {
			return nil, &files.DuplicateBasenameError{
				Name: entry.Name, Paths: []string{pathByName[entry.Name], entry.Path},
			}
		}
		sourceByName[entry.Name] = entry.ID
		pathByName[entry.Name] = entry.Path
	}
	return server, nil
}

func (s *Server) Serve(ctx context.Context, link transport.Duplex) error {
	for {
		header, err := readHeader(ctx, link)
		if err != nil {
			return err
		}
		if header.Type != CommandTypeRequest {
			return fmt.Errorf("%w: command type %d", ErrProtocol, header.Type)
		}

		switch header.Command {
		case CommandExit:
			return writeHeader(ctx, link, Header{Type: CommandTypeResponse, Command: CommandExit})
		case CommandList:
			if err := s.serveList(ctx, link); err != nil {
				return err
			}
		case CommandFileRange:
			if err := s.serveRange(ctx, link, header.PayloadSize); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %d", ErrUnsupportedCommand, header.Command)
		}
	}
}

func (s *Server) serveRange(ctx context.Context, link transport.Duplex, payloadSize uint32) error {
	if payloadSize < 16 || payloadSize > MaxRequestPayloadSize {
		return fmt.Errorf("%w: invalid range payload size %d", ErrProtocol, payloadSize)
	}
	if err := writeHeader(ctx, link, Header{
		Type:        CommandTypeAcknowledgement,
		Command:     CommandFileRange,
		PayloadSize: payloadSize,
	}); err != nil {
		return err
	}
	payload := make([]byte, payloadSize)
	if err := transport.ReadFull(ctx, link, payload); err != nil {
		return err
	}
	request, err := ParseRangeRequest(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	sourceID, ok := s.sourceByName[request.Name]
	if !ok {
		return fmt.Errorf("open range %q offset=%d size=%d: %w", request.Name, request.Offset, request.Size, files.ErrFileNotFound)
	}
	reader, availableSize, err := s.catalog.OpenRange(sourceID, request.Offset, uint64(request.Size))
	if err != nil {
		return fmt.Errorf("open range %q offset=%d size=%d: %w", request.Name, request.Offset, request.Size, err)
	}
	defer reader.Close()
	s.reporter.Requested(sourceID, request.Offset, uint64(request.Size))

	if err := writeHeader(ctx, link, Header{
		Type:        CommandTypeResponse,
		Command:     CommandFileRange,
		PayloadSize: request.Size,
	}); err != nil {
		return err
	}
	acknowledgement, err := readHeader(ctx, link)
	if err != nil {
		return err
	}
	if acknowledgement.Type != CommandTypeAcknowledgement || acknowledgement.Command != CommandFileRange {
		return fmt.Errorf("%w: invalid file range acknowledgement", ErrProtocol)
	}

	const chunkSize = 1 << 20
	buffer := make([]byte, min(uint64(availableSize), chunkSize))
	remaining := uint64(availableSize)
	offset := request.Offset
	for remaining > 0 {
		current := min(remaining, uint64(len(buffer)))
		chunk := buffer[:current]
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
	return nil
}

func (s *Server) serveList(ctx context.Context, link transport.Duplex) error {
	var builder strings.Builder
	for _, entry := range s.catalog.Entries() {
		builder.WriteString(entry.Name)
		builder.WriteByte('\n')
	}
	payload := []byte(builder.String())
	if err := writeHeader(ctx, link, Header{
		Type:        CommandTypeResponse,
		Command:     CommandList,
		PayloadSize: uint32(len(payload)),
	}); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	acknowledgement, err := readHeader(ctx, link)
	if err != nil {
		return err
	}
	if acknowledgement.Type != CommandTypeAcknowledgement || acknowledgement.Command != CommandList {
		return fmt.Errorf("%w: invalid list acknowledgement", ErrProtocol)
	}
	return transport.WriteFull(ctx, link, payload)
}

func readHeader(ctx context.Context, link transport.Duplex) (Header, error) {
	encoded := make([]byte, HeaderSize)
	if err := transport.ReadFull(ctx, link, encoded); err != nil {
		return Header{}, err
	}
	header, err := DecodeHeader(encoded)
	if err != nil {
		return Header{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return header, nil
}

func writeHeader(ctx context.Context, link transport.Duplex, header Header) error {
	encoded := EncodeHeader(header)
	return transport.WriteFull(ctx, link, encoded[:])
}
