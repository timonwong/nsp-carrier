package protocoltrace

import "sync"

const MaxRecords = 300

type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

// Record contains payload-safe protocol metadata. It deliberately has no raw
// byte, local path, or wire-name field.
type Record struct {
	Sequence         uint64    `json:"sequence"`
	Direction        Direction `json:"direction"`
	Operation        string    `json:"operation"`
	Command          uint32    `json:"command"`
	PayloadBytes     uint64    `json:"payloadBytes"`
	SourceID         string    `json:"sourceId,omitempty"`
	Offset           uint64    `json:"offset,omitempty"`
	Size             uint64    `json:"size,omitempty"`
	ResultCode       uint32    `json:"resultCode,omitempty"`
	HasResult        bool      `json:"hasResult,omitempty"`
	Index            uint32    `json:"index,omitempty"`
	IntegrityChecked bool      `json:"integrityChecked"`
	IntegrityValid   bool      `json:"integrityValid"`
}

type Reporter interface {
	Report(Record)
}

type ReporterFunc func(Record)

func (report ReporterFunc) Report(record Record) {
	if report != nil {
		report(record)
	}
}

type Buffer struct {
	mu        sync.Mutex
	next      uint64
	records   []Record
	truncated bool
}

func (b *Buffer) Report(record Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.records) >= MaxRecords {
		b.truncated = true
		return
	}
	b.next++
	record.Sequence = b.next
	b.records = append(b.records, record)
}

func (b *Buffer) Snapshot() ([]Record, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Record(nil), b.records...), b.truncated
}
