package goldleaf_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

type pinnedTranscripts struct {
	References []string `json:"references"`
	BlockSize  int      `json:"blockSize"`
	ZeroPadded bool     `json:"zeroPadded"`
	Catalog    struct {
		Name       string `json:"name"`
		ContentHex string `json:"contentHex"`
	} `json:"catalog"`
	Exchanges []struct {
		Name               string `json:"name"`
		RequestPrefixHex   string `json:"requestPrefixHex"`
		ResponsePrefixHex  string `json:"responsePrefixHex"`
		ResponsePayloadHex string `json:"responsePayloadHex"`
	} `json:"exchanges"`
}

func TestServerMatchesPinnedGoldleafCommandTranscripts(t *testing.T) {
	fixture := loadPinnedTranscripts(t)
	content, err := hex.DecodeString(fixture.Catalog.ContentHex)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), fixture.Catalog.Name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	var input, want []byte
	for _, exchange := range fixture.Exchanges {
		input = append(input, paddedHexBlock(t, fixture.BlockSize, exchange.RequestPrefixHex)...)
		want = append(want, paddedHexBlock(t, fixture.BlockSize, exchange.ResponsePrefixHex)...)
		payload, err := hex.DecodeString(exchange.ResponsePayloadHex)
		if err != nil {
			t.Fatalf("%s response payload: %v", exchange.Name, err)
		}
		want = append(want, payload...)
	}
	var warnings []goldleaf.Warning
	server, err := goldleaf.NewServer(catalog, host.NewProgress(catalog, host.ProfileGoldleaf), func(warning goldleaf.Warning) {
		warnings = append(warnings, warning)
	})
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input, transport.WithMaxRead(37), transport.WithMaxWrite(41))
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output differs from pinned transcript")
	}
	if len(warnings) != 1 || warnings[0].Operation != "delete" {
		t.Fatalf("mutation warnings = %#v", warnings)
	}
}

func loadPinnedTranscripts(t testing.TB) pinnedTranscripts {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "pinned-command-transcripts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture pinnedTranscripts
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.References) != 2 || fixture.BlockSize != goldleaf.BlockSize || !fixture.ZeroPadded {
		t.Fatalf("invalid pinned transcript metadata: %#v", fixture)
	}
	return fixture
}

func paddedHexBlock(t testing.TB, blockSize int, prefixHex string) []byte {
	t.Helper()
	prefix, err := hex.DecodeString(prefixHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) > blockSize {
		t.Fatalf("transcript prefix is %d bytes, block is %d", len(prefix), blockSize)
	}
	block := make([]byte, blockSize)
	copy(block, prefix)
	return block
}
