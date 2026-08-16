package awoo_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/awoo"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestServerMatchesPinnedDifferentialTranscript(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "pinned-*-transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 2 {
		t.Fatalf("pinned fixture count = %d, want 2", len(fixtures))
	}
	for _, fixturePath := range fixtures {
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			testPinnedTranscript(t, fixturePath)
		})
	}
}

func testPinnedTranscript(t *testing.T, fixturePath string) {
	t.Helper()
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		References      []string `json:"references"`
		DeviceToHostHex string   `json:"deviceToHostHex"`
		HostToDeviceHex string   `json:"hostToDeviceHex"`
	}
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.References) != 2 {
		t.Fatalf("pinned references = %q", fixture.References)
	}
	input, err := hex.DecodeString(fixture.DeviceToHostHex)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(fixture.HostToDeviceHex)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	server, err := awoo.NewServer(catalog, host.NewProgress(catalog))
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input)
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output = %x, want pinned %x", link.Written(), want)
	}
}

func TestServerMatchesPinnedListAndRangeTranscriptsAcrossShortIO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []awoo.CommandID{awoo.CommandFileRange, awoo.CommandFileRangeAlternative} {
		t.Run(commandName(command), func(t *testing.T) {
			payload := rangePayload("game.nsp", 3, 4)
			rangeHeader := awoo.EncodeCommandHeader(awoo.CommandHeader{
				Type: awoo.CommandTypeRequest, Command: command, DataSize: uint64(len(payload)),
			})
			exitHeader := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
			input := append(append(rangeHeader[:], payload...), exitHeader[:]...)
			link := transport.NewMemory(input, transport.WithMaxRead(3), transport.WithMaxWrite(5))
			progress := host.NewProgress(catalog)
			server, err := awoo.NewServer(catalog, progress)
			if err != nil {
				t.Fatal(err)
			}
			if err := server.Serve(context.Background(), link); err != nil {
				t.Fatalf("Serve() error = %v", err)
			}

			list := []byte("game.nsp\n")
			listHeader := awoo.EncodeListHeader(uint32(len(list)))
			response := awoo.EncodeCommandHeader(awoo.CommandHeader{
				Type: awoo.CommandTypeResponse, Command: awoo.CommandFileRange, DataSize: 4,
			})
			want := append(append(append(listHeader[:], list...), response[:]...), []byte("3456")...)
			if !bytes.Equal(link.Written(), want) {
				t.Fatalf("wire output = %x, want %x", link.Written(), want)
			}
			snapshot := progress.Snapshots(true, false)[catalog.Entries()[0].ID]
			if snapshot.UniqueServedBytes != 4 || snapshot.WireBytes != 4 || snapshot.RangeRequests != 1 {
				t.Fatalf("progress = %#v", snapshot)
			}
		})
	}
}

func TestServerRejectsMalformedUnknownAndOutOfBoundsRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "malformed magic", input: make([]byte, awoo.CommandHeaderSize), wantErr: awoo.ErrProtocol},
		{name: "unknown command", input: commandBytes(awoo.CommandID(99), nil), wantErr: awoo.ErrUnsupportedCommand},
		{name: "missing name", input: commandBytes(awoo.CommandFileRange, rangePayload("missing.nsp", 0, 1)), wantErr: files.ErrFileNotFound},
		{name: "range beyond source", input: commandBytes(awoo.CommandFileRange, rangePayload("game.nsp", 100, 1)), wantErr: files.ErrRangeOutOfBounds},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, err := awoo.NewServer(catalog, host.NewProgress(catalog))
			if err != nil {
				t.Fatal(err)
			}
			err = server.Serve(context.Background(), transport.NewMemory(test.input))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Serve() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestServerHandlesTimeoutCancellationDisconnectAndSourceChanges(t *testing.T) {
	emptyCatalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
	t.Run("idle timeout", func(t *testing.T) {
		server, err := awoo.NewServer(emptyCatalog, host.NewProgress(emptyCatalog))
		if err != nil {
			t.Fatal(err)
		}
		link := transport.NewMemory(exit[:], transport.WithReadFaults(transport.ErrTimeout))
		if err := server.Serve(context.Background(), link); err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		server, err := awoo.NewServer(emptyCatalog, host.NewProgress(emptyCatalog))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := server.Serve(ctx, transport.NewMemory(nil)); !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() error = %v, want context.Canceled", err)
		}
	})
	t.Run("disconnect", func(t *testing.T) {
		server, err := awoo.NewServer(emptyCatalog, host.NewProgress(emptyCatalog))
		if err != nil {
			t.Fatal(err)
		}
		if err := server.Serve(context.Background(), transport.NewMemory(nil)); !errors.Is(err, transport.ErrDisconnected) {
			t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
		}
	})
	t.Run("source changed", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "game.nsp")
		if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		server, err := awoo.NewServer(catalog, host.NewProgress(catalog))
		if err != nil {
			t.Fatal(err)
		}
		input := commandBytes(awoo.CommandFileRange, rangePayload("game.nsp", 0, 1))
		if err := server.Serve(context.Background(), transport.NewMemory(input)); !errors.Is(err, files.ErrSourceChanged) {
			t.Fatalf("Serve() error = %v, want ErrSourceChanged", err)
		}
	})
}

func TestServerServesOffsetsBeyondFourGiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.xci")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	const offset = int64(1<<32 + 7)
	if err := file.Truncate(offset + 4); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("tail"), offset); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	payload := rangePayload("large.xci", uint64(offset), 4)
	exit := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
	input := append(commandBytes(awoo.CommandFileRange, payload), exit[:]...)
	server, err := awoo.NewServer(catalog, host.NewProgress(catalog))
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input)
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !bytes.HasSuffix(link.Written(), []byte("tail")) {
		t.Fatalf("wire output does not end in sparse-file tail: %x", link.Written())
	}
}

func commandBytes(command awoo.CommandID, payload []byte) []byte {
	header := awoo.EncodeCommandHeader(awoo.CommandHeader{
		Type: awoo.CommandTypeRequest, Command: command, DataSize: uint64(len(payload)),
	})
	return append(header[:], payload...)
}

func commandName(command awoo.CommandID) string {
	if command == awoo.CommandFileRangeAlternative {
		return "alternative"
	}
	return "default"
}
