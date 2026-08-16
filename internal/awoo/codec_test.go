package awoo_test

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/awoo"
)

func TestCommandHeaderMatchesPinnedAwooTranscript(t *testing.T) {
	want := []byte{
		'T', 'U', 'C', '0', 0, 0, 0, 0,
		1, 0, 0, 0,
		40, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	header := awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandFileRange, DataSize: 40}
	encoded := awoo.EncodeCommandHeader(header)
	if string(encoded[:]) != string(want) {
		t.Fatalf("EncodeCommandHeader() = %x, want %x", encoded, want)
	}
	decoded, err := awoo.DecodeCommandHeader(want)
	if err != nil || decoded != header {
		t.Fatalf("DecodeCommandHeader() = %#v, %v", decoded, err)
	}
}

func TestEncodeFileListValidatesBeforeAllocatingPayload(t *testing.T) {
	for _, name := range []string{"line\nbreak.nsp", `back\slash.nsp`, string([]byte{0xff}) + ".nsp"} {
		if _, _, err := awoo.EncodeFileList([]string{name}); !errors.Is(err, awoo.ErrInvalidWireName) {
			t.Fatalf("unsafe list error = %v", err)
		}
	}
	name := strings.Repeat("a", awoo.MaxNameSize)
	names := make([]string, awoo.MaxListPayloadSize/(len(name)+1)+1)
	for index := range names {
		names[index] = name
	}
	if _, _, err := awoo.EncodeFileList(names); !errors.Is(err, awoo.ErrInvalidList) {
		t.Fatalf("oversized list error = %v", err)
	}
}

func TestParseRangeRequestSupports64BitOffsetsAndLengths(t *testing.T) {
	name := "large.xci"
	payload := make([]byte, awoo.RangeMetadataSize+len(name))
	binary.LittleEndian.PutUint64(payload[0:8], 1<<33+9)
	binary.LittleEndian.PutUint64(payload[8:16], 1<<32+7)
	binary.LittleEndian.PutUint64(payload[16:24], uint64(len(name)))
	copy(payload[awoo.RangeMetadataSize:], name)
	request, err := awoo.ParseRangeRequest(payload)
	if err != nil || request.Size != 1<<33+9 || request.Offset != 1<<32+7 || request.Name != name {
		t.Fatalf("ParseRangeRequest() = %#v, %v", request, err)
	}
}

func TestCodecRejectsMalformedMagicLengthsAndUnsafeNames(t *testing.T) {
	if _, err := awoo.DecodeCommandHeader(make([]byte, awoo.CommandHeaderSize)); !errors.Is(err, awoo.ErrInvalidHeader) {
		t.Fatalf("DecodeCommandHeader() error = %v", err)
	}
	for _, payload := range [][]byte{
		make([]byte, awoo.RangeMetadataSize-1),
		rangePayload("", 0, 1),
		rangePayload("../game.nsp", 0, 1),
	} {
		if _, err := awoo.ParseRangeRequest(payload); !errors.Is(err, awoo.ErrInvalidRangeRequest) {
			t.Fatalf("ParseRangeRequest(%x) error = %v", payload, err)
		}
	}
	header := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest})
	header[20] = 1
	if _, err := awoo.DecodeCommandHeader(header[:]); !errors.Is(err, awoo.ErrInvalidHeader) {
		t.Fatalf("reserved header error = %v", err)
	}
}

func FuzzDecodeCommandHeader(f *testing.F) {
	valid := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandFileRange, DataSize: 32})
	f.Add(valid[:])
	f.Add(make([]byte, awoo.CommandHeaderSize))
	for _, input := range pinnedDeviceInputs(f) {
		f.Add(input[:awoo.CommandHeaderSize])
	}
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = awoo.DecodeCommandHeader(data) })
}

func FuzzParseRangeRequest(f *testing.F) {
	f.Add(rangePayload("game.nsp", 3, 4))
	f.Add([]byte{})
	for _, input := range pinnedDeviceInputs(f) {
		payloadSize := binary.LittleEndian.Uint64(input[12:20])
		f.Add(input[awoo.CommandHeaderSize : awoo.CommandHeaderSize+int(payloadSize)])
	}
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = awoo.ParseRangeRequest(data) })
}

func pinnedDeviceInputs(t testing.TB) [][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "pinned-*-transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([][]byte, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct {
			DeviceToHostHex string `json:"deviceToHostHex"`
		}
		if err := json.Unmarshal(content, &fixture); err != nil {
			t.Fatal(err)
		}
		input, err := hex.DecodeString(fixture.DeviceToHostHex)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func rangePayload(name string, offset, size uint64) []byte {
	payload := make([]byte, awoo.RangeMetadataSize+len(name))
	binary.LittleEndian.PutUint64(payload[0:8], size)
	binary.LittleEndian.PutUint64(payload[8:16], offset)
	binary.LittleEndian.PutUint64(payload[16:24], uint64(len(name)))
	copy(payload[awoo.RangeMetadataSize:], name)
	return payload
}
