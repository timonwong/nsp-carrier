package awoo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	CommandHeaderSize  = 32
	RangeMetadataSize  = 32
	MaxNameSize        = 4 << 10
	MaxCommandDataSize = RangeMetadataSize + MaxNameSize
	MaxListPayloadSize = 4 << 20
)

var (
	CommandMagic           = [4]byte{'T', 'U', 'C', '0'}
	ListMagic              = [4]byte{'T', 'U', 'L', '0'}
	ErrInvalidHeader       = errors.New("invalid Awoo USB command header")
	ErrInvalidList         = errors.New("invalid Awoo USB file list")
	ErrInvalidRangeRequest = errors.New("invalid Awoo USB range request")
	ErrInvalidWireName     = errors.New("invalid Awoo USB wire name")
)

type CommandType uint8

const (
	CommandTypeRequest CommandType = iota
	CommandTypeResponse
)

type CommandID uint32

const (
	CommandExit CommandID = iota
	CommandFileRange
	CommandFileRangeAlternative
)

type CommandHeader struct {
	Type     CommandType
	Command  CommandID
	DataSize uint64
}

func EncodeCommandHeader(header CommandHeader) [CommandHeaderSize]byte {
	var encoded [CommandHeaderSize]byte
	copy(encoded[0:4], CommandMagic[:])
	encoded[4] = byte(header.Type)
	binary.LittleEndian.PutUint32(encoded[8:12], uint32(header.Command))
	binary.LittleEndian.PutUint64(encoded[12:20], header.DataSize)
	return encoded
}

func DecodeCommandHeader(encoded []byte) (CommandHeader, error) {
	if len(encoded) != CommandHeaderSize || string(encoded[0:4]) != string(CommandMagic[:]) {
		return CommandHeader{}, ErrInvalidHeader
	}
	if !zeroBytes(encoded[5:8]) || !zeroBytes(encoded[20:32]) {
		return CommandHeader{}, fmt.Errorf("%w: non-zero reserved bytes", ErrInvalidHeader)
	}
	header := CommandHeader{
		Type: CommandType(encoded[4]), Command: CommandID(binary.LittleEndian.Uint32(encoded[8:12])),
		DataSize: binary.LittleEndian.Uint64(encoded[12:20]),
	}
	if header.Type != CommandTypeRequest && header.Type != CommandTypeResponse {
		return CommandHeader{}, fmt.Errorf("%w: command type %d", ErrInvalidHeader, header.Type)
	}
	return header, nil
}

func EncodeListHeader(size uint32) [16]byte {
	var encoded [16]byte
	copy(encoded[0:4], ListMagic[:])
	binary.LittleEndian.PutUint32(encoded[4:8], size)
	return encoded
}

func EncodeFileList(names []string) ([16]byte, []byte, error) {
	var total uint64
	for _, name := range names {
		if err := ValidateWireName(name); err != nil {
			return [16]byte{}, nil, fmt.Errorf("%w: %w", ErrInvalidList, err)
		}
		total += uint64(len(name) + 1)
		if total > MaxListPayloadSize {
			return [16]byte{}, nil, fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalidList, MaxListPayloadSize)
		}
	}
	var builder strings.Builder
	builder.Grow(int(total))
	for _, name := range names {
		builder.WriteString(name)
		builder.WriteByte('\n')
	}
	payload := []byte(builder.String())
	return EncodeListHeader(uint32(len(payload))), payload, nil
}

func ValidateWireName(name string) error {
	if len(name) == 0 || len(name) > MaxNameSize || !utf8.ValidString(name) ||
		name == "." || name == ".." || strings.ContainsAny(name, "\x00\r\n/\\") {
		return ErrInvalidWireName
	}
	return nil
}

type RangeRequest struct {
	Size   uint64
	Offset uint64
	Name   string
}

func ParseRangeRequest(payload []byte) (RangeRequest, error) {
	if len(payload) < RangeMetadataSize || len(payload) > MaxCommandDataSize {
		return RangeRequest{}, ErrInvalidRangeRequest
	}
	nameSize := binary.LittleEndian.Uint64(payload[16:24])
	if nameSize == 0 || nameSize > MaxNameSize || nameSize != uint64(len(payload)-RangeMetadataSize) {
		return RangeRequest{}, fmt.Errorf("%w: name length %d", ErrInvalidRangeRequest, nameSize)
	}
	if !zeroBytes(payload[24:32]) {
		return RangeRequest{}, fmt.Errorf("%w: non-zero reserved bytes", ErrInvalidRangeRequest)
	}
	nameBytes := payload[RangeMetadataSize:]
	name := string(nameBytes)
	if err := ValidateWireName(name); err != nil {
		return RangeRequest{}, fmt.Errorf("%w: %w", ErrInvalidRangeRequest, err)
	}
	return RangeRequest{
		Size:   binary.LittleEndian.Uint64(payload[0:8]),
		Offset: binary.LittleEndian.Uint64(payload[8:16]),
		Name:   name,
	}, nil
}

func zeroBytes(values []byte) bool {
	for _, value := range values {
		if value != 0 {
			return false
		}
	}
	return true
}
