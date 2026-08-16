package dbi_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestServerListsFrozenCatalogAndExitsAcrossShortIO(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"base.nsp", "update.nsz"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog([]string{root}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	listRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandList})
	listAck := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeAcknowledgement, Command: dbi.CommandList})
	exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input := append(append(listRequest[:], listAck[:]...), exitRequest[:]...)
	link := transport.NewMemory(input, transport.WithMaxRead(3), transport.WithMaxWrite(5))

	server, _ := newServer(t, catalog)
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	payload := []byte("base.nsp\nupdate.nsz\n")
	listResponse := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeResponse,
		Command:     dbi.CommandList,
		PayloadSize: uint32(len(payload)),
	})
	exitResponse := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeResponse, Command: dbi.CommandExit})
	want := append(append(listResponse[:], payload...), exitResponse[:]...)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("written bytes = %x, want %x", link.Written(), want)
	}
}

func TestServerRejectsDuplicateBasenamesInItsWireProjection(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "left", "game.nsp"),
		filepath.Join(root, "right", "game.nsp"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog(paths, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	_, err = dbi.NewServer(catalog, nil)
	if !errors.Is(err, files.ErrDuplicateBasename) {
		t.Fatalf("NewServer() error = %v, want ErrDuplicateBasename", err)
	}
}

func TestServerHandlesTimeoutCancellationDisconnectAndMalformedFrames(t *testing.T) {
	emptyCatalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("idle timeout", func(t *testing.T) {
		exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
		link := transport.NewMemory(exitRequest[:], transport.WithReadFaults(transport.ErrTimeout))
		server, _ := newServer(t, emptyCatalog)
		if err := server.Serve(context.Background(), link); err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		server, _ := newServer(t, emptyCatalog)
		err := server.Serve(ctx, transport.NewMemory(nil))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() error = %v, want context.Canceled", err)
		}
	})

	t.Run("disconnect", func(t *testing.T) {
		server, _ := newServer(t, emptyCatalog)
		err := server.Serve(context.Background(), transport.NewMemory(nil))
		if !errors.Is(err, transport.ErrDisconnected) {
			t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
		}
	})

	t.Run("malformed frame", func(t *testing.T) {
		server, _ := newServer(t, emptyCatalog)
		err := server.Serve(context.Background(), transport.NewMemory(make([]byte, dbi.HeaderSize)))
		if !errors.Is(err, dbi.ErrProtocol) {
			t.Fatalf("Serve() error = %v, want ErrProtocol", err)
		}
	})

	t.Run("unsupported command", func(t *testing.T) {
		request := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandID(99)})
		server, _ := newServer(t, emptyCatalog)
		err := server.Serve(context.Background(), transport.NewMemory(request[:]))
		if !errors.Is(err, dbi.ErrUnsupportedCommand) {
			t.Fatalf("Serve() error = %v, want ErrUnsupportedCommand", err)
		}
	})
}

func TestServerServesFileRangeAndRecordsProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	detail := make([]byte, 16+len("game.nsp"))
	binary.LittleEndian.PutUint32(detail[0:4], 4)
	binary.LittleEndian.PutUint64(detail[4:12], 3)
	binary.LittleEndian.PutUint32(detail[12:16], uint32(len("game.nsp")))
	copy(detail[16:], "game.nsp")
	request := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeRequest,
		Command:     dbi.CommandFileRange,
		PayloadSize: uint32(len(detail)),
	})
	finalAck := dbi.EncodeHeader(dbi.Header{
		Type:    dbi.CommandTypeAcknowledgement,
		Command: dbi.CommandFileRange,
	})
	exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input := append(append(append(request[:], detail...), finalAck[:]...), exitRequest[:]...)
	link := transport.NewMemory(input, transport.WithMaxRead(5), transport.WithMaxWrite(2))

	server, progress := newServer(t, catalog)
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	requestAck := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeAcknowledgement,
		Command:     dbi.CommandFileRange,
		PayloadSize: uint32(len(detail)),
	})
	response := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeResponse,
		Command:     dbi.CommandFileRange,
		PayloadSize: 4,
	})
	exitResponse := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeResponse, Command: dbi.CommandExit})
	want := append(append(append(requestAck[:], response[:]...), []byte("3456")...), exitResponse[:]...)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("written bytes = %x, want %x", link.Written(), want)
	}

	value := progress.Snapshots(false, false)[catalog.Entries()[0].ID]
	if value.UniqueServedBytes != 4 || value.WireBytes != 4 {
		t.Fatalf("progress = %#v", value)
	}
}

func TestServerServesAvailableTailWhenAlignedRangeCrossesEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	detail := make([]byte, 16+len("game.nsp"))
	binary.LittleEndian.PutUint32(detail[0:4], 4)
	binary.LittleEndian.PutUint64(detail[4:12], 8)
	binary.LittleEndian.PutUint32(detail[12:16], uint32(len("game.nsp")))
	copy(detail[16:], "game.nsp")
	request := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeRequest,
		Command:     dbi.CommandFileRange,
		PayloadSize: uint32(len(detail)),
	})
	finalAck := dbi.EncodeHeader(dbi.Header{
		Type:    dbi.CommandTypeAcknowledgement,
		Command: dbi.CommandFileRange,
	})
	exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input := append(append(append(request[:], detail...), finalAck[:]...), exitRequest[:]...)
	link := transport.NewMemory(input)

	server, progress := newServer(t, catalog)
	if err := server.Serve(context.Background(), link); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	requestAck := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeAcknowledgement,
		Command:     dbi.CommandFileRange,
		PayloadSize: uint32(len(detail)),
	})
	response := dbi.EncodeHeader(dbi.Header{
		Type:        dbi.CommandTypeResponse,
		Command:     dbi.CommandFileRange,
		PayloadSize: 4,
	})
	exitResponse := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeResponse, Command: dbi.CommandExit})
	want := append(append(append(requestAck[:], response[:]...), []byte("89")...), exitResponse[:]...)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("written bytes = %x, want %x", link.Written(), want)
	}

	value := progress.Snapshots(false, false)[catalog.Entries()[0].ID]
	if value.UniqueServedBytes != 2 || value.WireBytes != 2 {
		t.Fatalf("progress = %#v", value)
	}
}

func TestServerRecordsRangeRequestOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	var input []byte
	for _, request := range []struct {
		offset uint64
		size   uint32
	}{
		{offset: 0, size: 4},
		{offset: 4, size: 4},
		{offset: 2, size: 2},
		{offset: 2, size: 2},
	} {
		detail := make([]byte, 16+len("game.nsp"))
		binary.LittleEndian.PutUint32(detail[0:4], request.size)
		binary.LittleEndian.PutUint64(detail[4:12], request.offset)
		binary.LittleEndian.PutUint32(detail[12:16], uint32(len("game.nsp")))
		copy(detail[16:], "game.nsp")
		header := dbi.EncodeHeader(dbi.Header{
			Type:        dbi.CommandTypeRequest,
			Command:     dbi.CommandFileRange,
			PayloadSize: uint32(len(detail)),
		})
		ack := dbi.EncodeHeader(dbi.Header{
			Type:    dbi.CommandTypeAcknowledgement,
			Command: dbi.CommandFileRange,
		})
		input = append(input, header[:]...)
		input = append(input, detail...)
		input = append(input, ack[:]...)
	}
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input = append(input, exit[:]...)

	server, progress := newServer(t, catalog)
	if err := server.Serve(context.Background(), transport.NewMemory(input)); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	value := progress.Snapshots(false, false)[catalog.Entries()[0].ID]
	if value.RangeRequests != 4 || value.NonSequentialRequests != 2 ||
		value.BackwardRequests != 1 || value.RepeatedRequests != 1 ||
		value.MaxRequestedOffset != 4 {
		t.Fatalf("request ordering = %#v", value)
	}
}

func newServer(t *testing.T, catalog *files.Catalog) (*dbi.Server, *host.Progress) {
	t.Helper()
	progress := host.NewProgress(catalog)
	server, err := dbi.NewServer(catalog, progress)
	if err != nil {
		t.Fatal(err)
	}
	return server, progress
}
