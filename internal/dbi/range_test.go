package dbi_test

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/timonwong/ya-dbibackend/internal/dbi"
)

func TestParseRangeRequestSupportsOffsetsBeyondFourGiB(t *testing.T) {
	payload, err := hex.DecodeString("2000000000000000010000000800000067616d652e6e7370")
	if err != nil {
		t.Fatal(err)
	}

	request, err := dbi.ParseRangeRequest(payload)
	if err != nil {
		t.Fatalf("ParseRangeRequest() error = %v", err)
	}
	if request.Size != 32 || request.Offset != 1<<32 || request.Name != "game.nsp" {
		t.Fatalf("ParseRangeRequest() = %#v", request)
	}
}

func TestParseRangeRequestRejectsMalformedOrUnsafeNames(t *testing.T) {
	requestPayload := func(name []byte, declaredLength uint32) []byte {
		payload := make([]byte, 16+len(name))
		binary.LittleEndian.PutUint32(payload[0:4], 1)
		binary.LittleEndian.PutUint32(payload[12:16], declaredLength)
		copy(payload[16:], name)
		return payload
	}

	tests := map[string][]byte{
		"short fixed fields":  make([]byte, 15),
		"length mismatch":     requestPayload([]byte("game.nsp"), 7),
		"invalid UTF-8":       requestPayload([]byte{0xff}, 1),
		"path separator":      requestPayload([]byte("dir/game.nsp"), 12),
		"dot path":            requestPayload([]byte(".."), 2),
		"NUL":                 requestPayload([]byte("game\x00.nsp"), 9),
		"name above four KiB": requestPayload([]byte(strings.Repeat("a", 4097)), 4097),
	}

	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := dbi.ParseRangeRequest(payload); err == nil {
				t.Fatal("ParseRangeRequest() error = nil")
			}
		})
	}
}

func FuzzParseRangeRequest(f *testing.F) {
	seed, err := hex.DecodeString("2000000000000000010000000800000067616d652e6e7370")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("short"))
	f.Fuzz(func(t *testing.T, payload []byte) {
		_, _ = dbi.ParseRangeRequest(payload)
	})
}
