package dbi_test

import (
	"testing"

	"github.com/timonwong/ya-dbibackend/internal/dbi"
)

func TestProgressSeparatesUniqueRangesFromWireBytes(t *testing.T) {
	progress := dbi.NewProgress(100)

	for _, served := range []struct {
		offset uint64
		size   uint32
	}{
		{offset: 20, size: 40},
		{offset: 0, size: 30},
		{offset: 50, size: 50},
		{offset: 20, size: 40},
	} {
		if err := progress.Record(served.offset, served.size); err != nil {
			t.Fatalf("Record(%d, %d) error = %v", served.offset, served.size, err)
		}
	}

	snapshot := progress.Snapshot()
	if snapshot.UniqueServedBytes != 100 {
		t.Fatalf("UniqueServedBytes = %d, want 100", snapshot.UniqueServedBytes)
	}
	if snapshot.WireBytes != 160 {
		t.Fatalf("WireBytes = %d, want 160", snapshot.WireBytes)
	}
	if snapshot.Percent != 100 {
		t.Fatalf("Percent = %v, want 100", snapshot.Percent)
	}
}
