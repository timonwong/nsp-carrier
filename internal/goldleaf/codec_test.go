package goldleaf_test

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/goldleaf"
)

func TestDriveCountCommandBlockMatchesPinnedGoldleafTranscript(t *testing.T) {
	input := make([]byte, goldleaf.BlockSize)
	copy(input[0:4], []byte("GLCI"))
	binary.LittleEndian.PutUint32(input[4:8], uint32(goldleaf.CommandGetDriveCount))

	request, err := goldleaf.DecodeRequest(input)
	if err != nil || request.Command != goldleaf.CommandGetDriveCount {
		t.Fatalf("DecodeRequest() = %#v, %v", request, err)
	}

	response := goldleaf.NewResponse(goldleaf.ResultSuccess)
	if err := response.WriteUint32(1); err != nil {
		t.Fatal(err)
	}
	encoded, err := response.Block()
	if err != nil {
		t.Fatal(err)
	}
	want := make([]byte, goldleaf.BlockSize)
	copy(want[0:4], []byte("GLCO"))
	binary.LittleEndian.PutUint32(want[8:12], 1)
	if string(encoded[:]) != string(want) {
		t.Fatalf("response = %x, want %x", encoded[:16], want[:16])
	}
}

func TestCodecRejectsMalformedBlocksAndBoundedStrings(t *testing.T) {
	if _, err := goldleaf.DecodeRequest(make([]byte, goldleaf.BlockSize-1)); !errors.Is(err, goldleaf.ErrInvalidBlock) {
		t.Fatalf("short DecodeRequest() error = %v", err)
	}
	invalidMagic := make([]byte, goldleaf.BlockSize)
	copy(invalidMagic[:4], "NOPE")
	if _, err := goldleaf.DecodeRequest(invalidMagic); !errors.Is(err, goldleaf.ErrInvalidBlock) {
		t.Fatalf("magic DecodeRequest() error = %v", err)
	}
	invalidString := make([]byte, goldleaf.BlockSize)
	copy(invalidString[:4], "GLCI")
	binary.LittleEndian.PutUint32(invalidString[4:8], uint32(goldleaf.CommandStatPath))
	binary.LittleEndian.PutUint32(invalidString[8:12], goldleaf.BlockSize)
	request, err := goldleaf.DecodeRequest(invalidString)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.ReadString(); !errors.Is(err, goldleaf.ErrInvalidBlock) {
		t.Fatalf("ReadString() error = %v", err)
	}

	response := goldleaf.NewResponse(goldleaf.ResultSuccess)
	if err := response.WriteString(string(make([]byte, goldleaf.BlockSize))); !errors.Is(err, goldleaf.ErrBlockOverflow) {
		t.Fatalf("WriteString() error = %v", err)
	}
}

func FuzzDecodeRequest(f *testing.F) {
	fixture := loadPinnedTranscripts(f)
	for _, exchange := range fixture.Exchanges {
		prefix, err := hex.DecodeString(exchange.RequestPrefixHex)
		if err != nil {
			f.Fatal(err)
		}
		block := make([]byte, fixture.BlockSize)
		copy(block, prefix)
		f.Add(block)
	}
	f.Add([]byte("GLCI"))
	f.Fuzz(func(t *testing.T, data []byte) {
		request, err := goldleaf.DecodeRequest(data)
		if err == nil {
			_, _ = request.ReadString()
			_, _ = request.ReadUint64()
			_, _ = request.ReadUint64()
		}
	})
}
