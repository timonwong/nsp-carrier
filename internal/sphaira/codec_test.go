package sphaira_test

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/sphaira"
)

func TestPacketEncodingMatchesPinnedSPH0WireContract(t *testing.T) {
	encoded := sphaira.EncodeCommand(sphaira.CommandOpen, 7, 0)
	wantPrefix := []byte{
		0x30, 0x48, 0x50, 0x53,
		0x01, 0x00, 0x00, 0x00,
		0x07, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	if string(encoded[:20]) != string(wantPrefix) {
		t.Fatalf("encoded prefix = %x, want %x", encoded[:20], wantPrefix)
	}
	if got, want := binary.LittleEndian.Uint32(encoded[20:]), crc32.Checksum(wantPrefix, crc32.MakeTable(crc32.Castagnoli)); got != want {
		t.Fatalf("header CRC32C = %08x, want %08x", got, want)
	}
	packet, err := sphaira.DecodePacket(encoded[:])
	if err != nil || packet.Command() != sphaira.CommandOpen || packet.Arg3 != 7 {
		t.Fatalf("DecodePacket() = %#v, %v", packet, err)
	}
}

func TestCastagnoliUsesTheStandardCheckVector(t *testing.T) {
	if got := sphaira.PayloadCRC32C([]byte("123456789")); got != 0xe3069283 {
		t.Fatalf("CRC32C = %08x, want e3069283", got)
	}
}

func TestDataPacketPreserves64BitOffset(t *testing.T) {
	const offset = uint64(1)<<32 + 0x89abcdef
	encoded := sphaira.EncodeData(offset, 4096, 0x10203040)
	packet, err := sphaira.DecodePacket(encoded[:])
	if err != nil {
		t.Fatal(err)
	}
	if packet.Offset() != offset || packet.Size() != 4096 || packet.PayloadCRC32C() != 0x10203040 {
		t.Fatalf("decoded data packet = %#v", packet)
	}
}

func TestFileInfoPacks48BitSizeAndFlags(t *testing.T) {
	arg3, arg4, err := sphaira.PackFileInfo(0x1234_89abcdef, sphaira.FlagNone)
	if err != nil {
		t.Fatal(err)
	}
	if arg3 != 0x1234 || arg4 != 0x89abcdef {
		t.Fatalf("packed file info = %08x %08x", arg3, arg4)
	}
	if _, _, err := sphaira.PackFileInfo(uint64(1)<<48, sphaira.FlagNone); !errors.Is(err, sphaira.ErrFileTooLarge) {
		t.Fatalf("oversized PackFileInfo() error = %v", err)
	}
}

func TestDecodePacketRejectsEveryUntrustedFrameShape(t *testing.T) {
	valid := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{name: "short", data: valid[:23], want: sphaira.ErrInvalidPacketLength},
		{name: "long", data: append(valid[:], 0), want: sphaira.ErrInvalidPacketLength},
		{name: "magic", data: withInvalidMagic(valid[:]), want: sphaira.ErrInvalidMagic},
		{name: "crc", data: mutate(valid[:], 8), want: sphaira.ErrInvalidHeaderCRC},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := sphaira.DecodePacket(test.data); !errors.Is(err, test.want) {
				t.Fatalf("DecodePacket() error = %v, want %v", err, test.want)
			}
		})
	}
}

func FuzzDecodePacket(f *testing.F) {
	valid := sphaira.EncodeCommand(sphaira.CommandOpen, 2, 0)
	f.Add(valid[:])
	f.Add([]byte("SPH0"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = sphaira.DecodePacket(data)
	})
}

func mutate(data []byte, index int) []byte {
	result := append([]byte(nil), data...)
	result[index] ^= 0xff
	return result
}

func withInvalidMagic(data []byte) []byte {
	result := mutate(data, 0)
	binary.LittleEndian.PutUint32(result[20:], crc32.Checksum(result[:20], crc32.MakeTable(crc32.Castagnoli)))
	return result
}
