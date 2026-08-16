package dbi

import (
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidRangeRequest = errors.New("invalid DBI0 file range request")
var ErrInvalidWireName = errors.New("invalid DBI0 wire name")

const (
	MaxRequestPayloadSize = 64 << 10
	MaxBasenameSize       = 4 << 10
)

type RangeRequest struct {
	Size   uint32
	Offset uint64
	Name   string
}

func ValidateWireName(name string) error {
	if len(name) == 0 || len(name) > MaxBasenameSize || !utf8.ValidString(name) ||
		name == "." || name == ".." || strings.ContainsAny(name, "\x00\r\n/\\") {
		return ErrInvalidWireName
	}
	return nil
}

func ParseRangeRequest(payload []byte) (RangeRequest, error) {
	if len(payload) < 16 || len(payload) > MaxRequestPayloadSize {
		return RangeRequest{}, ErrInvalidRangeRequest
	}

	nameLength := binary.LittleEndian.Uint32(payload[12:16])
	if uint64(nameLength) != uint64(len(payload)-16) {
		return RangeRequest{}, ErrInvalidRangeRequest
	}

	nameBytes := payload[16:]
	name := string(nameBytes)
	if ValidateWireName(name) != nil {
		return RangeRequest{}, ErrInvalidRangeRequest
	}

	return RangeRequest{
		Size:   binary.LittleEndian.Uint32(payload[0:4]),
		Offset: binary.LittleEndian.Uint64(payload[4:12]),
		Name:   name,
	}, nil
}
