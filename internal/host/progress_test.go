package host_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

func TestProgressSeparatesUniqueRangesFromWireBytesAndOwnsFileState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := host.NewProgress(catalog, host.ProfileDBI)
	progress.Requested(sourceID, 20, 40)
	for _, served := range []struct {
		offset uint64
		size   uint32
	}{
		{offset: 20, size: 40},
		{offset: 0, size: 30},
		{offset: 50, size: 50},
		{offset: 20, size: 40},
	} {
		if err := progress.Served(sourceID, served.offset, served.size); err != nil {
			t.Fatalf("Served(%d, %d) error = %v", served.offset, served.size, err)
		}
	}

	snapshot := progress.Snapshots(false, false)[sourceID]
	if snapshot.UniqueServedBytes != 100 || snapshot.WireBytes != 160 || snapshot.Percent != 100 || snapshot.State != host.FileFullyServed {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	failedSessionSnapshot := progress.Snapshots(true, true)[sourceID]
	if failedSessionSnapshot.State != host.FileFullyServed {
		t.Fatalf("fully served file after session failure = %q, want %q", failedSessionSnapshot.State, host.FileFullyServed)
	}
}

func TestProgressMarksEveryRequestedByteServedWithoutRequiringTheWholeSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := host.NewProgress(catalog, host.ProfileAwoo)
	progress.Requested(sourceID, 0, 2)
	if err := progress.Served(sourceID, 0, 2); err != nil {
		t.Fatal(err)
	}
	if state := progress.Snapshots(true, false)[sourceID].State; state != host.FileFullyServed {
		t.Fatalf("terminal partial-source state = %q, want %q", state, host.FileFullyServed)
	}
}

func TestProgressMarksAnIncompleteRequestedRangeInterrupted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := host.NewProgress(catalog, host.ProfileAwoo)
	progress.Requested(sourceID, 0, 4)
	if err := progress.Served(sourceID, 0, 2); err != nil {
		t.Fatal(err)
	}
	if state := progress.Snapshots(true, false)[sourceID].State; state != host.FileInterrupted {
		t.Fatalf("terminal incomplete-request state = %q, want %q", state, host.FileInterrupted)
	}
}

func TestProgressClipsDBIAlignedTailRequestsToTheFrozenSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := host.NewProgress(catalog, host.ProfileDBI)
	progress.Requested(sourceID, 0, 8)
	if err := progress.Served(sourceID, 0, 8); err != nil {
		t.Fatal(err)
	}
	progress.Requested(sourceID, 8, 4)
	if err := progress.Served(sourceID, 8, 2); err != nil {
		t.Fatal(err)
	}
	if state := progress.Snapshots(true, false)[sourceID].State; state != host.FileFullyServed {
		t.Fatalf("terminal aligned-tail state = %q, want %q", state, host.FileFullyServed)
	}
}

func TestProgressKeepsGoldleafPartialReadsPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := host.NewProgress(catalog, host.ProfileGoldleaf)
	progress.Requested(sourceID, 0, 2)
	if err := progress.Served(sourceID, 0, 2); err != nil {
		t.Fatal(err)
	}
	if state := progress.Snapshots(true, false)[sourceID].State; state != host.FileInterrupted {
		t.Fatalf("terminal Goldleaf partial-read state = %q, want %q", state, host.FileInterrupted)
	}
}
