package dbi_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/ya-dbibackend/internal/dbi"
	"github.com/timonwong/ya-dbibackend/internal/files"
	"github.com/timonwong/ya-dbibackend/internal/transport"
)

func TestServerListsFrozenCatalogAndExitsAcrossShortIO(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"base.nsp", "update.nsz"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog([]string{root})
	if err != nil {
		t.Fatal(err)
	}

	listRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandList})
	listAck := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeAcknowledgement, Command: dbi.CommandList})
	exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input := append(append(listRequest[:], listAck[:]...), exitRequest[:]...)
	link := transport.NewMemory(input, transport.WithMaxRead(3), transport.WithMaxWrite(5))

	server := dbi.NewServer(catalog)
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

func TestServerHandlesTimeoutCancellationDisconnectAndMalformedFrames(t *testing.T) {
	emptyCatalog, err := files.BuildCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("idle timeout", func(t *testing.T) {
		exitRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
		link := transport.NewMemory(exitRequest[:], transport.WithReadFaults(transport.ErrTimeout))
		if err := dbi.NewServer(emptyCatalog).Serve(context.Background(), link); err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := dbi.NewServer(emptyCatalog).Serve(ctx, transport.NewMemory(nil))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() error = %v, want context.Canceled", err)
		}
	})

	t.Run("disconnect", func(t *testing.T) {
		err := dbi.NewServer(emptyCatalog).Serve(context.Background(), transport.NewMemory(nil))
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Serve() error = %v, want io.EOF", err)
		}
	})

	t.Run("malformed frame", func(t *testing.T) {
		err := dbi.NewServer(emptyCatalog).Serve(context.Background(), transport.NewMemory(make([]byte, dbi.HeaderSize)))
		if !errors.Is(err, dbi.ErrProtocol) {
			t.Fatalf("Serve() error = %v, want ErrProtocol", err)
		}
	})
}

func TestServerServesFileRangeAndRecordsProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path})
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

	server := dbi.NewServer(catalog)
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

	progress, ok := server.Progress("game.nsp")
	if !ok || progress.UniqueServedBytes != 4 || progress.WireBytes != 4 {
		t.Fatalf("Progress() = %#v, %v", progress, ok)
	}
}
