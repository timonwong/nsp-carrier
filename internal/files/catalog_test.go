package files_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/ya-dbibackend/internal/files"
)

func TestBuildCatalogRecursesFiltersAndSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"base.NSP":                            "base",
		filepath.Join("nested", "update.nsz"): "update",
		"notes.txt":                           "ignore",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "base.NSP"), filepath.Join(root, "linked.xci")); err != nil {
		t.Fatal(err)
	}

	catalog, err := files.BuildCatalog([]string{root, filepath.Join(root, "base.NSP")})
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}
	entries := catalog.Entries()
	if len(entries) != 2 {
		t.Fatalf("len(Entries()) = %d, want 2: %#v", len(entries), entries)
	}
	if entries[0].Name != "base.NSP" || entries[1].Name != "update.nsz" {
		t.Fatalf("Entries() names = %q, %q", entries[0].Name, entries[1].Name)
	}
	if entries[0].ID == "" || entries[0].Path == "" {
		t.Fatalf("Entries()[0] lacks stable identity: %#v", entries[0])
	}
}

func TestCatalogOpenRangeReadsFrozenSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path})
	if err != nil {
		t.Fatal(err)
	}

	reader, err := catalog.OpenRange("game.nsp", 3, 4)
	if err != nil {
		t.Fatalf("OpenRange() error = %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "3456" {
		t.Fatalf("range content = %q, want %q", got, "3456")
	}
}

func TestCatalogOpenRangeRejectsChangedOrOutOfBoundsSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := catalog.OpenRange("game.nsp", 9, 2); !errors.Is(err, files.ErrRangeOutOfBounds) {
		t.Fatalf("out-of-bounds OpenRange() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.OpenRange("game.nsp", 0, 1); !errors.Is(err, files.ErrSourceChanged) {
		t.Fatalf("changed-source OpenRange() error = %v", err)
	}
}

func TestCatalogOpenRangeSupportsOffsetsBeyondFourGiB(t *testing.T) {
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

	catalog, err := files.BuildCatalog([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := catalog.OpenRange("large.xci", uint64(offset), 4)
	if err != nil {
		t.Fatalf("OpenRange() error = %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "tail" {
		t.Fatalf("range content = %q, want tail", got)
	}
}

func TestBuildCatalogRejectsDuplicateBasenames(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.Mkdir(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(right, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(left, "game.nsp"), filepath.Join(right, "game.nsp")} {
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := files.BuildCatalog([]string{left, right})
	if !errors.Is(err, files.ErrDuplicateBasename) {
		t.Fatalf("BuildCatalog() error = %v, want ErrDuplicateBasename", err)
	}
}
