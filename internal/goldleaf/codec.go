package goldleaf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const BlockSize = 0x1000

const MaxWireNameBytes = BlockSize - 8 - 4 - len("VIRT:/") - 8 - 8

var (
	ErrInvalidBlock  = errors.New("invalid Goldleaf command block")
	ErrBlockOverflow = errors.New("Goldleaf command block overflow")
)

func ValidateWireName(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > MaxWireNameBytes ||
		!utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n/\\") {
		return ErrInvalidBlock
	}
	return nil
}

type CommandID uint32

const (
	CommandGetDriveCount CommandID = iota + 1
	CommandGetDriveInfo
	CommandStatPath
	CommandGetFileCount
	CommandGetFile
	CommandGetDirectoryCount
	CommandGetDirectory
	CommandStartFile
	CommandReadFile
	CommandWriteFile
	CommandEndFile
	CommandCreate
	CommandDelete
	CommandRename
	CommandGetSpecialPathCount
	CommandGetSpecialPath
	CommandSelectFile
)

type ResultCode uint32

const (
	ResultSuccess            ResultCode = 0
	ResultExceptionCaught    ResultCode = 0xBAF1
	ResultInvalidIndex       ResultCode = 0xBAF2
	ResultInvalidFileMode    ResultCode = 0xBAF3
	ResultSelectionCancelled ResultCode = 0xBAF4
)

type PathType uint32

const (
	PathTypeInvalid PathType = iota
	PathTypeFile
	PathTypeDirectory
)

type FileMode uint32

const (
	FileModeRead FileMode = iota + 1
	FileModeWrite
	FileModeAppend
)

type Request struct {
	Command CommandID
	block   []byte
	pos     int
}

func DecodeRequest(block []byte) (*Request, error) {
	if len(block) != BlockSize || string(block[0:4]) != "GLCI" {
		return nil, ErrInvalidBlock
	}
	command := CommandID(binary.LittleEndian.Uint32(block[4:8]))
	if command == 0 {
		return nil, fmt.Errorf("%w: command ID 0", ErrInvalidBlock)
	}
	return &Request{Command: command, block: block, pos: 8}, nil
}

func (r *Request) ReadUint32() (uint32, error) {
	data, err := r.read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

func (r *Request) ReadUint64() (uint64, error) {
	data, err := r.read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

func (r *Request) ReadString() (string, error) {
	length, err := r.ReadUint32()
	if err != nil || length > BlockSize {
		return "", fmt.Errorf("%w: invalid string length", ErrInvalidBlock)
	}
	data, err := r.read(int(length))
	if err != nil || !utf8.Valid(data) {
		return "", fmt.Errorf("%w: invalid UTF-8 string", ErrInvalidBlock)
	}
	return string(data), nil
}

func (r *Request) read(size int) ([]byte, error) {
	if size < 0 || r.pos > len(r.block)-size {
		return nil, ErrInvalidBlock
	}
	data := r.block[r.pos : r.pos+size]
	r.pos += size
	return data, nil
}

type Response struct {
	block [BlockSize]byte
	pos   int
	err   error
}

func NewResponse(result ResultCode) *Response {
	response := &Response{pos: 8}
	copy(response.block[0:4], []byte("GLCO"))
	binary.LittleEndian.PutUint32(response.block[4:8], uint32(result))
	return response
}

func (r *Response) WriteUint32(value uint32) error {
	data, err := r.reserve(4)
	if err == nil {
		binary.LittleEndian.PutUint32(data, value)
	}
	return err
}

func (r *Response) WriteUint64(value uint64) error {
	data, err := r.reserve(8)
	if err == nil {
		binary.LittleEndian.PutUint64(data, value)
	}
	return err
}

func (r *Response) WriteString(value string) error {
	if !utf8.ValidString(value) {
		return r.fail(ErrInvalidBlock)
	}
	if err := r.WriteUint32(uint32(len(value))); err != nil {
		return err
	}
	data, err := r.reserve(len(value))
	if err == nil {
		copy(data, value)
	}
	return err
}

func (r *Response) Block() ([BlockSize]byte, error) { return r.block, r.err }

func (r *Response) reserve(size int) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if size < 0 || r.pos > len(r.block)-size {
		return nil, r.fail(ErrBlockOverflow)
	}
	data := r.block[r.pos : r.pos+size]
	r.pos += size
	return data, nil
}

func (r *Response) fail(err error) error {
	if r.err == nil {
		r.err = err
	}
	return r.err
}
