package protocoltrace_test

import (
	"testing"

	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
)

func TestBufferAssignsSequenceAndReturnsIndependentSnapshots(t *testing.T) {
	var buffer protocoltrace.Buffer
	buffer.Report(protocoltrace.Record{
		Direction: protocoltrace.Inbound,
		Operation: "file_range",
		Command:   2,
		SourceID:  "source-1",
		Offset:    4096,
		Size:      8192,
	})

	records, truncated := buffer.Snapshot()
	if truncated || len(records) != 1 {
		t.Fatalf("Snapshot() = %#v, %t", records, truncated)
	}
	if records[0].Sequence != 1 || records[0].Command != 2 || records[0].SourceID != "source-1" {
		t.Fatalf("record = %#v", records[0])
	}
	records[0].Operation = "mutated"
	again, _ := buffer.Snapshot()
	if again[0].Operation != "file_range" {
		t.Fatalf("snapshot aliases buffer: %#v", again[0])
	}
}

func TestBufferIsBoundedAndReportsTruncation(t *testing.T) {
	var buffer protocoltrace.Buffer
	for index := 0; index < protocoltrace.MaxRecords+10; index++ {
		buffer.Report(protocoltrace.Record{Operation: "request"})
	}
	records, truncated := buffer.Snapshot()
	if !truncated {
		t.Fatal("Snapshot() did not report truncation")
	}
	if len(records) != protocoltrace.MaxRecords {
		t.Fatalf("records = %d, want %d", len(records), protocoltrace.MaxRecords)
	}
	if records[len(records)-1].Sequence != protocoltrace.MaxRecords {
		t.Fatalf("last sequence = %d", records[len(records)-1].Sequence)
	}
}
