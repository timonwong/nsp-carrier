package dbi

import (
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidRangeRequest = errors.New("invalid DBI0 file range request")

const (
	MaxRequestPayloadSize = 64 << 10
	MaxBasenameSize       = 4 << 10
)

type RangeRequest struct {
	Size   uint32
	Offset uint64
	Name   string
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
	if len(nameBytes) == 0 || len(nameBytes) > MaxBasenameSize ||
		!utf8.Valid(nameBytes) || name == "." || name == ".." ||
		strings.ContainsAny(name, "\x00/\\") {
		return RangeRequest{}, ErrInvalidRangeRequest
	}

	return RangeRequest{
		Size:   binary.LittleEndian.Uint32(payload[0:4]),
		Offset: binary.LittleEndian.Uint64(payload[4:12]),
		Name:   name,
	}, nil
}
