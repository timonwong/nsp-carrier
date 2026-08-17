package sphaira

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

const (
	Magic        uint32 = 0x53504830
	PacketSize          = 24
	MaxRangeSize uint32 = 16 << 20
	MaxFileSize  uint64 = 1<<48 - 1
)

type Command uint32

const (
	CommandQuit Command = iota
	CommandOpen
)

type Result uint32

const (
	ResultOK Result = iota
	ResultError
)

type Flags uint16

const (
	FlagNone   Flags = 0
	FlagStream Flags = 1
)

var (
	ErrInvalidPacketLength = errors.New("invalid SPH0 packet length")
	ErrInvalidMagic        = errors.New("invalid SPH0 magic")
	ErrInvalidHeaderCRC    = errors.New("invalid SPH0 header CRC32C")
	ErrFileTooLarge        = errors.New("SPH0 source exceeds 48-bit size limit")
)

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

type Packet struct {
	Arg2 uint32
	Arg3 uint32
	Arg4 uint32
	Arg5 uint32
}

func (p Packet) Command() Command      { return Command(p.Arg2) }
func (p Packet) Offset() uint64        { return uint64(p.Arg2)<<32 | uint64(p.Arg3) }
func (p Packet) Size() uint32          { return p.Arg4 }
func (p Packet) PayloadCRC32C() uint32 { return p.Arg5 }

func PayloadCRC32C(payload []byte) uint32 { return crc32.Checksum(payload, castagnoli) }

func EncodeCommand(command Command, arg3, arg4 uint32) [PacketSize]byte {
	return encode(Packet{Arg2: uint32(command), Arg3: arg3, Arg4: arg4})
}

func EncodeData(offset uint64, size uint32, payloadCRC uint32) [PacketSize]byte {
	return encode(Packet{Arg2: uint32(offset >> 32), Arg3: uint32(offset), Arg4: size, Arg5: payloadCRC})
}

func EncodeResult(result Result, arg3, arg4 uint32) [PacketSize]byte {
	return encode(Packet{Arg2: uint32(result), Arg3: arg3, Arg4: arg4})
}

func encode(packet Packet) [PacketSize]byte {
	var encoded [PacketSize]byte
	binary.LittleEndian.PutUint32(encoded[0:4], Magic)
	binary.LittleEndian.PutUint32(encoded[4:8], packet.Arg2)
	binary.LittleEndian.PutUint32(encoded[8:12], packet.Arg3)
	binary.LittleEndian.PutUint32(encoded[12:16], packet.Arg4)
	binary.LittleEndian.PutUint32(encoded[16:20], packet.Arg5)
	binary.LittleEndian.PutUint32(encoded[20:24], PayloadCRC32C(encoded[:20]))
	return encoded
}

func DecodePacket(encoded []byte) (Packet, error) {
	if len(encoded) != PacketSize {
		return Packet{}, ErrInvalidPacketLength
	}
	if binary.LittleEndian.Uint32(encoded[20:24]) != PayloadCRC32C(encoded[:20]) {
		return Packet{}, ErrInvalidHeaderCRC
	}
	if binary.LittleEndian.Uint32(encoded[0:4]) != Magic {
		return Packet{}, ErrInvalidMagic
	}
	return Packet{
		Arg2: binary.LittleEndian.Uint32(encoded[4:8]),
		Arg3: binary.LittleEndian.Uint32(encoded[8:12]),
		Arg4: binary.LittleEndian.Uint32(encoded[12:16]),
		Arg5: binary.LittleEndian.Uint32(encoded[16:20]),
	}, nil
}

func PackFileInfo(size uint64, flags Flags) (uint32, uint32, error) {
	if size > MaxFileSize {
		return 0, 0, ErrFileTooLarge
	}
	return uint32(size>>32) | uint32(flags)<<16, uint32(size), nil
}

func ValidateSourceSize(size int64) error {
	if size < 0 || uint64(size) > MaxFileSize {
		return ErrFileTooLarge
	}
	return nil
}
