package sphaira_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/sphaira"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestServerMatchesPinnedCleanRoomInstallTranscript(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "pinned-install-transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		BehavioralReference string `json:"behavioralReference"`
		Derivation          string `json:"derivation"`
		Catalog             struct {
			Name       string `json:"name"`
			ContentHex string `json:"contentHex"`
		} `json:"catalog"`
		DeviceToHostHex string `json:"deviceToHostHex"`
		HostToDeviceHex string `json:"hostToDeviceHex"`
	}
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.BehavioralReference != "NaGaa95/sphaira@3f8303db00f33bfffa83ce0a1b750a1de14656e2" || fixture.Derivation == "" {
		t.Fatalf("incomplete transcript provenance: %#v", fixture)
	}
	fileContent, err := hex.DecodeString(fixture.Catalog.ContentHex)
	if err != nil {
		t.Fatal(err)
	}
	input, err := hex.DecodeString(fixture.DeviceToHostHex)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(fixture.HostToDeviceHex)
	if err != nil {
		t.Fatal(err)
	}
	server, err := sphaira.NewServer(sphairaCatalog(t, fixture.Catalog.Name, fileContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input, transport.WithMaxRead(5), transport.WithMaxWrite(7))
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output = %x\nwant = %x", link.Written(), want)
	}
}

func TestServerRejectsMalformedFramesWithoutReply(t *testing.T) {
	catalog := sphairaCatalog(t, "game.nsp", []byte("content"))
	server, err := sphaira.NewServer(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	valid := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	malformed := append([]byte(nil), valid[:]...)
	malformed[20] ^= 0xff
	link := transport.NewMemory(malformed)
	if err := server.Serve(context.Background(), link); !errors.Is(err, sphaira.ErrProtocol) || !errors.Is(err, sphaira.ErrInvalidHeaderCRC) {
		t.Fatalf("Serve() error = %v", err)
	}
	if len(link.Written()) != 0 {
		t.Fatalf("malformed frame received reply: %x", link.Written())
	}
}

func TestFileCloseDoesNotCompleteWithoutFinalQuit(t *testing.T) {
	catalog := sphairaCatalog(t, "game.nsp", []byte("content"))
	server, err := sphaira.NewServer(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
	closePacket := sphaira.EncodeData(0, 0, 0)
	input := append(append(handshake[:], open[:]...), closePacket[:]...)
	if err := server.Serve(context.Background(), transport.NewMemory(input)); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want disconnect while waiting for QUIT", err)
	}
}

func TestServerAcceptsZeroLengthReadAwayFromClose(t *testing.T) {
	catalog := sphairaCatalog(t, "game.xci", []byte("content"))
	server, err := sphaira.NewServer(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
	zeroLengthRead := sphaira.EncodeData(1, 0, 0)
	closePacket := sphaira.EncodeData(0, 0, 0)
	quit := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	input := append(append(append(append(handshake[:], open[:]...), zeroLengthRead[:]...), closePacket[:]...), quit[:]...)
	link := transport.NewMemory(input)

	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatal(err)
	}

	list := []byte("game.xci\n")
	filenameList := sphaira.EncodeResult(sphaira.ResultOK, uint32(len(list)), 0)
	openResult := sphaira.EncodeResult(sphaira.ResultOK, 0, uint32(len("content")))
	emptyReadResult := sphaira.EncodeResult(sphaira.ResultOK, 0, sphaira.PayloadCRC32C(nil))
	closeAck := sphaira.EncodeResult(sphaira.ResultOK, 0, 0)
	quitAck := sphaira.EncodeResult(sphaira.ResultOK, 0, 0)
	want := append(append(append(append(append(filenameList[:], list...), openResult[:]...), emptyReadResult[:]...), closeAck[:]...), quitAck[:]...)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output = %x\nwant = %x", link.Written(), want)
	}
}

func TestServerRepliesErrorThenTerminatesForSemanticFailures(t *testing.T) {
	tests := []struct {
		name    string
		request [sphaira.PacketSize]byte
	}{
		{name: "unknown command", request: sphaira.EncodeCommand(99, 0, 0)},
		{name: "invalid index", request: sphaira.EncodeCommand(sphaira.CommandOpen, 1, 0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := sphairaCatalog(t, "game.nsp", []byte("content"))
			server, err := sphaira.NewServer(catalog, nil)
			if err != nil {
				t.Fatal(err)
			}
			handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
			input := append(handshake[:], test.request[:]...)
			link := transport.NewMemory(input)
			if err := server.Serve(context.Background(), link); !errors.Is(err, sphaira.ErrProtocol) {
				t.Fatalf("Serve() error = %v", err)
			}
			list := []byte("game.nsp\n")
			wantSuffix := sphaira.EncodeResult(sphaira.ResultError, 0, 0)
			if !bytes.HasSuffix(link.Written(), wantSuffix[:]) || !bytes.Contains(link.Written(), list) {
				t.Fatalf("wire output = %x", link.Written())
			}
		})
	}
}

func TestServerRejectsInvalidRangesAndSourceMutationWithResultError(t *testing.T) {
	tests := []struct {
		name    string
		request [sphaira.PacketSize]byte
		mutate  bool
	}{
		{name: "oversized range", request: sphaira.EncodeData(0, sphaira.MaxRangeSize+1, 0)},
		{name: "offset beyond EOF", request: sphaira.EncodeData(8, 1, 0)},
		{name: "unexpected request checksum", request: sphaira.EncodeData(0, 1, 1)},
		{name: "source mutation", request: sphaira.EncodeData(0, 1, 0), mutate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "game.nsp")
			if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
			if err != nil {
				t.Fatal(err)
			}
			if test.mutate {
				if err := os.WriteFile(path, []byte("changed!"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			server, err := sphaira.NewServer(catalog, nil)
			if err != nil {
				t.Fatal(err)
			}
			handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
			open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
			input := append(append(handshake[:], open[:]...), test.request[:]...)
			link := transport.NewMemory(input)
			if err := server.Serve(context.Background(), link); !errors.Is(err, sphaira.ErrProtocol) {
				t.Fatalf("Serve() error = %v", err)
			}
			want := sphaira.EncodeResult(sphaira.ResultError, 0, 0)
			if !bytes.HasSuffix(link.Written(), want[:]) {
				t.Fatalf("wire output = %x", link.Written())
			}
		})
	}
}

func TestServerTraceContainsOnlyBoundedIntegrityMetadata(t *testing.T) {
	catalog := sphairaCatalog(t, "private-name.nsp", []byte("secret-payload"))
	var trace protocoltrace.Buffer
	server, err := sphaira.NewServerWithTrace(catalog, nil, &trace)
	if err != nil {
		t.Fatal(err)
	}
	malformed := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	malformed[20] ^= 0xff
	_ = server.Serve(context.Background(), transport.NewMemory(malformed[:]))
	records, _ := trace.Snapshot()
	if len(records) != 1 || !records[0].IntegrityChecked || records[0].IntegrityValid {
		t.Fatalf("trace = %#v", records)
	}
	encoded := []byte(fmt.Sprintf("%#v", records))
	for _, secret := range [][]byte{[]byte("private-name"), []byte("secret-payload")} {
		if bytes.Contains(encoded, secret) {
			t.Fatalf("trace leaked %q: %s", secret, encoded)
		}
	}
}

func sphairaCatalog(t testing.TB, name string, content []byte) *files.Catalog {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}
