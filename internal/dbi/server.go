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
	catalog  *files.Catalog
	progress map[string]*Progress
}

func NewServer(catalog *files.Catalog) *Server {
	progress := make(map[string]*Progress)
	for _, entry := range catalog.Entries() {
		progress[entry.Name] = NewProgress(uint64(entry.Size))
	}
	return &Server{catalog: catalog, progress: progress}
}

func (s *Server) Progress(name string) (ProgressSnapshot, bool) {
	progress, ok := s.progress[name]
	if !ok {
		return ProgressSnapshot{}, false
	}
	return progress.Snapshot(), true
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
	if err := readFull(ctx, link, payload); err != nil {
		return err
	}
	request, err := ParseRangeRequest(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	reader, availableSize, err := s.catalog.OpenRange(request.Name, request.Offset, request.Size)
	if err != nil {
		return fmt.Errorf("open range %q offset=%d size=%d: %w", request.Name, request.Offset, request.Size, err)
	}
	defer reader.Close()

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

	progress := s.progress[request.Name]
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
		written, err := writeFullCount(ctx, link, chunk)
		if written > 0 {
			if progressErr := progress.Record(offset, uint32(written)); progressErr != nil {
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
	return writeFull(ctx, link, payload)
}

func readHeader(ctx context.Context, link transport.Duplex) (Header, error) {
	encoded := make([]byte, HeaderSize)
	if err := readFull(ctx, link, encoded); err != nil {
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
	return writeFull(ctx, link, encoded[:])
}

func readFull(ctx context.Context, link transport.Duplex, destination []byte) error {
	for len(destination) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, err := link.Read(ctx, destination)
		if read > 0 {
			destination = destination[read:]
		}
		if errors.Is(err, transport.ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}
		if read == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

func writeFull(ctx context.Context, link transport.Duplex, source []byte) error {
	_, err := writeFullCount(ctx, link, source)
	return err
}

func writeFullCount(ctx context.Context, link transport.Duplex, source []byte) (int, error) {
	total := 0
	for len(source) > 0 {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		written, err := link.Write(ctx, source)
		if written > 0 {
			total += written
			source = source[written:]
		}
		if err != nil {
			return total, err
		}
		if written == 0 {
			return total, io.ErrNoProgress
		}
	}
	return total, nil
}
