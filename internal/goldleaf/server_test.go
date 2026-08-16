package goldleaf_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestServerExposesOnlyFrozenCatalogAsVirtualDrive(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "base.nsp"), filepath.Join(root, "update.nsp")}
	for index, path := range paths {
		if err := os.WriteFile(path, bytes.Repeat([]byte{byte(index + 1)}, index+2), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog(paths, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}

	input := bytes.Join([][]byte{
		requestBlock(goldleaf.CommandGetDriveCount),
		requestBlock(goldleaf.CommandGetDriveInfo, uint32Arg(0)),
		requestBlock(goldleaf.CommandGetFileCount, stringArg("VIRT:/")),
		requestBlock(goldleaf.CommandGetFile, stringArg("VIRT:/"), uint32Arg(0)),
		requestBlock(goldleaf.CommandGetFile, stringArg("VIRT:/"), uint32Arg(1)),
		requestBlock(goldleaf.CommandGetDirectoryCount, stringArg("VIRT:/")),
		requestBlock(goldleaf.CommandStatPath, stringArg("VIRT:/base.nsp")),
		requestBlock(goldleaf.CommandStatPath, stringArg("HOME:/secret")),
	}, nil)
	link := transport.NewMemory(input, transport.WithMaxRead(31), transport.WithMaxWrite(29))
	server, err := goldleaf.NewServer(catalog, host.NewProgress(catalog, host.ProfileGoldleaf), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}

	want := bytes.Join([][]byte{
		responseBlock(goldleaf.ResultSuccess, uint32Arg(1)),
		responseBlock(goldleaf.ResultSuccess, stringArg("Virtual"), stringArg("VIRT"), uint64Arg(5), uint64Arg(0)),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(2)),
		responseBlock(goldleaf.ResultSuccess, stringArg("base.nsp")),
		responseBlock(goldleaf.ResultSuccess, stringArg("update.nsp")),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(0)),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(uint32(goldleaf.PathTypeFile)), uint64Arg(2)),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(uint32(goldleaf.PathTypeInvalid)), uint64Arg(0)),
	}, nil)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output mismatch\n got: %x\nwant: %x", link.Written(), want)
	}
}

func TestServerServesExactReadRangesAndTracksWholeSourceCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	progress := host.NewProgress(catalog, host.ProfileGoldleaf)
	input := bytes.Join([][]byte{
		requestBlock(goldleaf.CommandStartFile, stringArg("VIRT:/game.nsp"), uint32Arg(uint32(goldleaf.FileModeRead))),
		requestBlock(goldleaf.CommandReadFile, stringArg("VIRT:/game.nsp"), uint64Arg(3), uint64Arg(4)),
		requestBlock(goldleaf.CommandEndFile, uint32Arg(uint32(goldleaf.FileModeRead))),
	}, nil)
	link := transport.NewMemory(input, transport.WithMaxRead(17), transport.WithMaxWrite(3))
	server, err := goldleaf.NewServer(catalog, progress, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}

	want := bytes.Join([][]byte{
		responseBlock(goldleaf.ResultSuccess),
		responseBlock(goldleaf.ResultSuccess, uint64Arg(4)),
		[]byte("3456"),
		responseBlock(goldleaf.ResultSuccess),
	}, nil)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output mismatch\n got: %x\nwant: %x", link.Written(), want)
	}
	snapshot := progress.Snapshots(false, false)[catalog.Entries()[0].ID]
	if snapshot.State != host.FileServing || snapshot.UniqueServedBytes != 4 || snapshot.RangeRequests != 1 {
		t.Fatalf("progress = %#v", snapshot)
	}
}

func TestServerServesOffsetsBeyondFourGiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.nsp")
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
	input := requestBlock(goldleaf.CommandReadFile, stringArg("VIRT:/large.nsp"), uint64Arg(uint64(offset)), uint64Arg(4))
	link := transport.NewMemory(input)
	server, err := goldleaf.NewServer(catalog, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}
	want := append(responseBlock(goldleaf.ResultSuccess, uint64Arg(4)), []byte("tail")...)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output does not preserve 64-bit offset")
	}
}

func TestServerRejectsMutationsWithWarningsAndContinuesSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.Join([][]byte{
		requestBlock(goldleaf.CommandWriteFile, stringArg("VIRT:/game.nsp"), uint64Arg(4)),
		[]byte("evil"),
		requestBlock(goldleaf.CommandCreate, stringArg("VIRT:/new.nsp"), uint32Arg(uint32(goldleaf.PathTypeFile))),
		requestBlock(goldleaf.CommandDelete, stringArg("VIRT:/game.nsp")),
		requestBlock(goldleaf.CommandRename, stringArg("VIRT:/game.nsp"), stringArg("renamed.nsp")),
		requestBlock(goldleaf.CommandGetDriveCount),
	}, nil)
	var warnings []goldleaf.Warning
	server, err := goldleaf.NewServer(catalog, nil, func(warning goldleaf.Warning) {
		warnings = append(warnings, warning)
	})
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input, transport.WithMaxRead(23), transport.WithMaxWrite(19))
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}

	want := bytes.Join([][]byte{
		responseBlock(goldleaf.ResultExceptionCaught),
		responseBlock(goldleaf.ResultExceptionCaught),
		responseBlock(goldleaf.ResultExceptionCaught),
		responseBlock(goldleaf.ResultExceptionCaught),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(1)),
	}, nil)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output mismatch after mutation rejection")
	}
	if len(warnings) != 4 {
		t.Fatalf("warnings = %#v, want four structured warnings", warnings)
	}
	for _, warning := range warnings {
		if warning.Operation == "" || warning.Code != "read-only-virtual-catalog" || warning.Message == "" {
			t.Fatalf("warning = %#v", warning)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "original" {
		t.Fatalf("source content = %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "new.nsp")); !os.IsNotExist(err) {
		t.Fatalf("create mutation changed host filesystem: %v", err)
	}
}

func TestServerKeepsVirtualNamespaceFlatAndHasNoHostPicker(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.Join([][]byte{
		requestBlock(goldleaf.CommandGetDirectory, stringArg("VIRT:/"), uint32Arg(0)),
		requestBlock(goldleaf.CommandGetSpecialPathCount),
		requestBlock(goldleaf.CommandGetSpecialPath, uint32Arg(0)),
		requestBlock(goldleaf.CommandSelectFile),
	}, nil)
	server, err := goldleaf.NewServer(catalog, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	link := transport.NewMemory(input)
	if err := server.Serve(context.Background(), link); !errors.Is(err, transport.ErrDisconnected) {
		t.Fatalf("Serve() error = %v, want ErrDisconnected", err)
	}
	want := bytes.Join([][]byte{
		responseBlock(goldleaf.ResultInvalidIndex),
		responseBlock(goldleaf.ResultSuccess, uint32Arg(0)),
		responseBlock(goldleaf.ResultInvalidIndex),
		responseBlock(goldleaf.ResultSelectionCancelled),
	}, nil)
	if !bytes.Equal(link.Written(), want) {
		t.Fatalf("wire output mismatch")
	}
}

func TestServerTreatsMalformedAndUnknownCommandsAsTerminal(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	server, err := goldleaf.NewServer(catalog, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "malformed magic", input: make([]byte, goldleaf.BlockSize), wantErr: goldleaf.ErrProtocol},
		{name: "unknown command", input: requestBlock(goldleaf.CommandID(99)), wantErr: goldleaf.ErrUnsupportedCommand},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := server.Serve(context.Background(), transport.NewMemory(test.input))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Serve() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestServerHandlesTimeoutCancellationAndSourceChanges(t *testing.T) {
	emptyCatalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	server, err := goldleaf.NewServer(emptyCatalog, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("timeout and short IO", func(t *testing.T) {
		link := transport.NewMemory(
			requestBlock(goldleaf.CommandGetDriveCount),
			transport.WithReadFaults(transport.ErrTimeout), transport.WithMaxRead(7), transport.WithMaxWrite(11),
		)
		err := server.Serve(context.Background(), link)
		if !errors.Is(err, transport.ErrDisconnected) || !bytes.Equal(link.Written(), responseBlock(goldleaf.ResultSuccess, uint32Arg(1))) {
			t.Fatalf("Serve() error = %v, output = %x", err, link.Written())
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := server.Serve(ctx, transport.NewMemory(nil)); !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() error = %v, want context.Canceled", err)
		}
	})
	t.Run("source change", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "game.nsp")
		if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("changed-length"), 0o644); err != nil {
			t.Fatal(err)
		}
		server, err := goldleaf.NewServer(catalog, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		input := requestBlock(goldleaf.CommandReadFile, stringArg("VIRT:/game.nsp"), uint64Arg(0), uint64Arg(1))
		if err := server.Serve(context.Background(), transport.NewMemory(input)); !errors.Is(err, files.ErrSourceChanged) {
			t.Fatalf("Serve() error = %v, want ErrSourceChanged", err)
		}
	})
	t.Run("range beyond source", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "game.nsp")
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		server, err := goldleaf.NewServer(catalog, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		input := requestBlock(goldleaf.CommandReadFile, stringArg("VIRT:/game.nsp"), uint64Arg(6), uint64Arg(8))
		if err := server.Serve(context.Background(), transport.NewMemory(input)); !errors.Is(err, files.ErrRangeOutOfBounds) {
			t.Fatalf("Serve() error = %v, want ErrRangeOutOfBounds", err)
		}
	})
}

func requestBlock(command goldleaf.CommandID, arguments ...[]byte) []byte {
	block := make([]byte, goldleaf.BlockSize)
	copy(block[:4], "GLCI")
	binary.LittleEndian.PutUint32(block[4:8], uint32(command))
	copyArguments(block[8:], arguments)
	return block
}

func responseBlock(result goldleaf.ResultCode, arguments ...[]byte) []byte {
	block := make([]byte, goldleaf.BlockSize)
	copy(block[:4], "GLCO")
	binary.LittleEndian.PutUint32(block[4:8], uint32(result))
	copyArguments(block[8:], arguments)
	return block
}

func copyArguments(destination []byte, arguments [][]byte) {
	for _, argument := range arguments {
		copied := copy(destination, argument)
		destination = destination[copied:]
	}
}

func uint32Arg(value uint32) []byte {
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, value)
	return encoded
}

func uint64Arg(value uint64) []byte {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, value)
	return encoded
}

func stringArg(value string) []byte { return append(uint32Arg(uint32(len(value))), value...) }
