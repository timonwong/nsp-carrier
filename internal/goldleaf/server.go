package goldleaf

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrProtocol           = errors.New("Goldleaf protocol error")
	ErrUnsupportedCommand = errors.New("unsupported Goldleaf command")
)

type RangeReporter interface {
	Requested(sourceID string, offset uint64, size uint64)
	Served(sourceID string, offset uint64, size uint32) error
}

type Warning struct {
	Operation string
	Code      string
	Message   string
}

type WarningReporter func(Warning)

type discardReporter struct{}

func (discardReporter) Requested(string, uint64, uint64)    {}
func (discardReporter) Served(string, uint64, uint32) error { return nil }

type Server struct {
	catalog      *files.Catalog
	entries      []files.Entry
	sourceByName map[string]string
	reporter     RangeReporter
	warn         WarningReporter
	totalSize    uint64
}

type wirePayload struct {
	reader   io.ReadCloser
	sourceID string
	offset   uint64
	size     uint64
}

func NewServer(catalog *files.Catalog, reporter RangeReporter, warn WarningReporter) (*Server, error) {
	if catalog == nil {
		return nil, errors.New("Goldleaf server requires a frozen catalog")
	}
	if reporter == nil {
		reporter = discardReporter{}
	}
	if warn == nil {
		warn = func(Warning) {}
	}
	server := &Server{
		catalog: catalog, entries: catalog.Entries(), sourceByName: make(map[string]string),
		reporter: reporter, warn: warn,
	}
	pathByName := make(map[string]string)
	for _, entry := range server.entries {
		if err := ValidateWireName(entry.Name); err != nil {
			return nil, fmt.Errorf("%w: invalid virtual catalog name %q", ErrProtocol, entry.Name)
		}
		if _, exists := server.sourceByName[entry.Name]; exists {
			return nil, &files.DuplicateBasenameError{
				Name: entry.Name, Paths: []string{pathByName[entry.Name], entry.Path},
			}
		}
		server.sourceByName[entry.Name] = entry.ID
		pathByName[entry.Name] = entry.Path
		server.totalSize += uint64(entry.Size)
	}
	return server, nil
}

func (s *Server) Serve(ctx context.Context, link transport.Duplex) error {
	for {
		block := make([]byte, BlockSize)
		if err := transport.ReadFull(ctx, link, block); err != nil {
			return err
		}
		request, err := DecodeRequest(block)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrProtocol, err)
		}
		var response *Response
		var payload *wirePayload
		if request.Command == CommandWriteFile {
			response, err = s.rejectWrite(ctx, link, request)
		} else {
			response, payload, err = s.handle(request)
		}
		if err != nil {
			return err
		}
		encoded, err := response.Block()
		if err != nil {
			return fmt.Errorf("%w: encode response: %v", ErrProtocol, err)
		}
		if err := transport.WriteFull(ctx, link, encoded[:]); err != nil {
			return err
		}
		if payload != nil {
			if err := s.writePayload(ctx, link, payload); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handle(request *Request) (*Response, *wirePayload, error) {
	switch request.Command {
	case CommandGetDriveCount:
		response := NewResponse(ResultSuccess)
		_ = response.WriteUint32(1)
		return response, nil, nil
	case CommandGetDriveInfo:
		return s.getDriveInfo(request)
	case CommandStatPath:
		return s.statPath(request)
	case CommandGetFileCount:
		return s.getFileCount(request)
	case CommandGetFile:
		return s.getFile(request)
	case CommandGetDirectoryCount:
		return s.getDirectoryCount(request)
	case CommandGetDirectory:
		return s.getDirectory(request)
	case CommandStartFile:
		return s.startFile(request)
	case CommandReadFile:
		return s.readFile(request)
	case CommandEndFile:
		return s.endFile(request)
	case CommandCreate:
		return s.rejectCreate(request)
	case CommandDelete:
		return s.rejectDelete(request)
	case CommandRename:
		return s.rejectRename(request)
	case CommandGetSpecialPathCount:
		response := NewResponse(ResultSuccess)
		_ = response.WriteUint32(0)
		return response, nil, nil
	case CommandGetSpecialPath:
		if _, err := request.ReadUint32(); err != nil {
			return nil, nil, s.protocolError("GetSpecialPath", err)
		}
		return NewResponse(ResultInvalidIndex), nil, nil
	case CommandSelectFile:
		return NewResponse(ResultSelectionCancelled), nil, nil
	default:
		return nil, nil, fmt.Errorf("%w: %d", ErrUnsupportedCommand, request.Command)
	}
}

func (s *Server) rejectWrite(ctx context.Context, link transport.Duplex, request *Request) (*Response, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, s.protocolError("WriteFile", err)
	}
	size, err := request.ReadUint64()
	if err != nil {
		return nil, s.protocolError("WriteFile", err)
	}
	if err := discardInput(ctx, link, size); err != nil {
		return nil, err
	}
	s.readOnlyWarning("write", path)
	return NewResponse(ResultExceptionCaught), nil
}

func (s *Server) rejectCreate(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("Create", err)
	}
	if _, err := request.ReadUint32(); err != nil {
		return nil, nil, s.protocolError("Create", err)
	}
	s.readOnlyWarning("create", path)
	return NewResponse(ResultExceptionCaught), nil, nil
}

func (s *Server) rejectDelete(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("Delete", err)
	}
	s.readOnlyWarning("delete", path)
	return NewResponse(ResultExceptionCaught), nil, nil
}

func (s *Server) rejectRename(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("Rename", err)
	}
	newName, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("Rename", err)
	}
	s.readOnlyWarning("rename", path+" -> "+newName)
	return NewResponse(ResultExceptionCaught), nil, nil
}

func (s *Server) readOnlyWarning(operation, target string) {
	s.warn(Warning{
		Operation: operation,
		Code:      "read-only-virtual-catalog",
		Message:   fmt.Sprintf("Goldleaf %s rejected for read-only virtual catalog target %q", operation, target),
	})
}

func discardInput(ctx context.Context, link transport.Duplex, size uint64) error {
	const chunkSize = 1 << 20
	buffer := make([]byte, min(size, chunkSize))
	remaining := size
	for remaining > 0 {
		current := min(remaining, uint64(len(buffer)))
		if err := transport.ReadFull(ctx, link, buffer[:int(current)]); err != nil {
			return err
		}
		remaining -= current
	}
	return nil
}

func (s *Server) getDriveInfo(request *Request) (*Response, *wirePayload, error) {
	index, err := request.ReadUint32()
	if err != nil {
		return nil, nil, s.protocolError("GetDriveInfo", err)
	}
	if index != 0 {
		return NewResponse(ResultInvalidIndex), nil, nil
	}
	response := NewResponse(ResultSuccess)
	_ = response.WriteString("Virtual")
	_ = response.WriteString("VIRT")
	_ = response.WriteUint64(s.totalSize)
	_ = response.WriteUint64(0)
	return response, nil, nil
}

func (s *Server) statPath(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("StatPath", err)
	}
	pathType, size := PathTypeInvalid, uint64(0)
	if path == "VIRT:/" {
		pathType = PathTypeDirectory
	} else if name, ok := virtualName(path); ok {
		for _, entry := range s.entries {
			if entry.Name == name {
				pathType, size = PathTypeFile, uint64(entry.Size)
				break
			}
		}
	}
	response := NewResponse(ResultSuccess)
	_ = response.WriteUint32(uint32(pathType))
	_ = response.WriteUint64(size)
	return response, nil, nil
}

func (s *Server) getFileCount(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("GetFileCount", err)
	}
	if path != "VIRT:/" {
		return NewResponse(ResultExceptionCaught), nil, nil
	}
	response := NewResponse(ResultSuccess)
	_ = response.WriteUint32(uint32(len(s.entries)))
	return response, nil, nil
}

func (s *Server) getFile(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("GetFile", err)
	}
	index, err := request.ReadUint32()
	if err != nil {
		return nil, nil, s.protocolError("GetFile", err)
	}
	if path != "VIRT:/" || uint64(index) >= uint64(len(s.entries)) {
		return NewResponse(ResultInvalidIndex), nil, nil
	}
	response := NewResponse(ResultSuccess)
	_ = response.WriteString(s.entries[index].Name)
	return response, nil, nil
}

func (s *Server) getDirectoryCount(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("GetDirectoryCount", err)
	}
	if path != "VIRT:/" {
		return NewResponse(ResultExceptionCaught), nil, nil
	}
	response := NewResponse(ResultSuccess)
	_ = response.WriteUint32(0)
	return response, nil, nil
}

func (s *Server) getDirectory(request *Request) (*Response, *wirePayload, error) {
	if _, err := request.ReadString(); err != nil {
		return nil, nil, s.protocolError("GetDirectory", err)
	}
	if _, err := request.ReadUint32(); err != nil {
		return nil, nil, s.protocolError("GetDirectory", err)
	}
	return NewResponse(ResultInvalidIndex), nil, nil
}

func (s *Server) startFile(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("StartFile", err)
	}
	mode, err := request.ReadUint32()
	if err != nil {
		return nil, nil, s.protocolError("StartFile", err)
	}
	if FileMode(mode) != FileModeRead {
		return NewResponse(ResultInvalidFileMode), nil, nil
	}
	if _, ok := s.sourceForPath(path); !ok {
		return NewResponse(ResultExceptionCaught), nil, nil
	}
	return NewResponse(ResultSuccess), nil, nil
}

func (s *Server) readFile(request *Request) (*Response, *wirePayload, error) {
	path, err := request.ReadString()
	if err != nil {
		return nil, nil, s.protocolError("ReadFile", err)
	}
	offset, err := request.ReadUint64()
	if err != nil {
		return nil, nil, s.protocolError("ReadFile", err)
	}
	size, err := request.ReadUint64()
	if err != nil {
		return nil, nil, s.protocolError("ReadFile", err)
	}
	sourceID, ok := s.sourceForPath(path)
	if !ok {
		return NewResponse(ResultExceptionCaught), nil, nil
	}
	reader, available, err := s.catalog.OpenRange(sourceID, offset, size)
	if err != nil {
		return nil, nil, fmt.Errorf("open Goldleaf range %q offset=%d size=%d: %w", path, offset, size, err)
	}
	if available != size {
		if reader != nil {
			_ = reader.Close()
		}
		return nil, nil, fmt.Errorf("open Goldleaf range %q offset=%d size=%d: %w", path, offset, size, files.ErrRangeOutOfBounds)
	}
	s.reporter.Requested(sourceID, offset, size)
	response := NewResponse(ResultSuccess)
	_ = response.WriteUint64(size)
	return response, &wirePayload{reader: reader, sourceID: sourceID, offset: offset, size: size}, nil
}

func (s *Server) endFile(request *Request) (*Response, *wirePayload, error) {
	mode, err := request.ReadUint32()
	if err != nil {
		return nil, nil, s.protocolError("EndFile", err)
	}
	if FileMode(mode) != FileModeRead {
		return NewResponse(ResultInvalidFileMode), nil, nil
	}
	return NewResponse(ResultSuccess), nil, nil
}

func (s *Server) writePayload(ctx context.Context, link transport.Duplex, payload *wirePayload) error {
	defer payload.reader.Close()
	const chunkSize = 1 << 20
	buffer := make([]byte, min(payload.size, chunkSize))
	remaining := payload.size
	offset := payload.offset
	for remaining > 0 {
		current := min(remaining, uint64(len(buffer)))
		chunk := buffer[:int(current)]
		if _, err := io.ReadFull(payload.reader, chunk); err != nil {
			return fmt.Errorf("%w: %v", files.ErrSourceChanged, err)
		}
		written, err := transport.WriteFullCount(ctx, link, chunk)
		if written > 0 {
			if progressErr := s.reporter.Served(payload.sourceID, offset, uint32(written)); progressErr != nil {
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

func (s *Server) protocolError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrProtocol, operation, err)
}

func virtualName(path string) (string, bool) {
	const prefix = "VIRT:/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return "", false
	}
	name := path[len(prefix):]
	for _, character := range name {
		if character == '/' || character == '\\' {
			return "", false
		}
	}
	return name, true
}

func (s *Server) sourceForPath(path string) (string, bool) {
	name, ok := virtualName(path)
	if !ok {
		return "", false
	}
	sourceID, ok := s.sourceByName[name]
	return sourceID, ok
}
