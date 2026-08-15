package dbi

import (
	"encoding/binary"
	"errors"
)

const HeaderSize = 16

var (
	Magic            = [4]byte{'D', 'B', 'I', '0'}
	ErrInvalidHeader = errors.New("invalid DBI0 header")
)

type CommandType uint32

const (
	CommandTypeRequest CommandType = iota
	CommandTypeResponse
	CommandTypeAcknowledgement
)

type CommandID uint32

const (
	CommandExit CommandID = iota
	CommandListLegacy
	CommandFileRange
	CommandList
)

type Header struct {
	Type        CommandType
	Command     CommandID
	PayloadSize uint32
}

func EncodeHeader(header Header) [HeaderSize]byte {
	var encoded [HeaderSize]byte
	copy(encoded[:4], Magic[:])
	binary.LittleEndian.PutUint32(encoded[4:8], uint32(header.Type))
	binary.LittleEndian.PutUint32(encoded[8:12], uint32(header.Command))
	binary.LittleEndian.PutUint32(encoded[12:16], header.PayloadSize)
	return encoded
}

func DecodeHeader(encoded []byte) (Header, error) {
	if len(encoded) != HeaderSize || string(encoded[:4]) != string(Magic[:]) {
		return Header{}, ErrInvalidHeader
	}
	return Header{
		Type:        CommandType(binary.LittleEndian.Uint32(encoded[4:8])),
		Command:     CommandID(binary.LittleEndian.Uint32(encoded[8:12])),
		PayloadSize: binary.LittleEndian.Uint32(encoded[12:16]),
	}, nil
}
