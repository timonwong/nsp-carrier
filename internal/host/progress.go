package host

import (
	"errors"
	"sync"

	"github.com/timonwong/nsp-carrier/internal/files"
)

var ErrProgressRangeOutOfBounds = errors.New("progress range out of bounds")

type FileState string

const (
	FileQueued       FileState = "Queued"
	FileRequested    FileState = "Requested"
	FileServing      FileState = "Serving"
	FileFullyServed  FileState = "FullyServed"
	FileNotRequested FileState = "NotRequested"
	FileInterrupted  FileState = "Interrupted"
	FileFailed       FileState = "Failed"
)

type interval struct{ start, end uint64 }

type fileProgress struct {
	total                 uint64
	intervals             []interval
	wireBytes             uint64
	rangeRequests         uint64
	nonSequentialRequests uint64
	backwardRequests      uint64
	repeatedRequests      uint64
	maxRequestedOffset    uint64
	lastRequestOffset     uint64
	lastRequestEnd        uint64
	hasRequest            bool
}

type Progress struct {
	mu    sync.Mutex
	files map[string]*fileProgress
}

func NewProgress(catalog *files.Catalog) *Progress {
	tracked := make(map[string]*fileProgress, len(catalog.Entries()))
	for _, entry := range catalog.Entries() {
		tracked[entry.ID] = &fileProgress{total: uint64(entry.Size)}
	}
	return &Progress{files: tracked}
}

func (p *Progress) Requested(sourceID string, offset uint64, size uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	value := p.files[sourceID]
	if value == nil {
		return
	}
	if value.hasRequest {
		if offset != value.lastRequestEnd {
			value.nonSequentialRequests++
		}
		if offset < value.lastRequestOffset {
			value.backwardRequests++
		}
		if offset == value.lastRequestOffset {
			value.repeatedRequests++
		}
	}
	value.rangeRequests++
	if offset > value.maxRequestedOffset {
		value.maxRequestedOffset = offset
	}
	value.lastRequestOffset = offset
	value.lastRequestEnd = offset + uint64(size)
	if value.lastRequestEnd < offset {
		value.lastRequestEnd = ^uint64(0)
	}
	value.hasRequest = true
}

func (p *Progress) Served(sourceID string, offset uint64, size uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	value := p.files[sourceID]
	if value == nil {
		return files.ErrFileNotFound
	}
	end := offset + uint64(size)
	if end < offset || offset > value.total || end > value.total {
		return ErrProgressRangeOutOfBounds
	}
	value.wireBytes += uint64(size)
	merged := interval{start: offset, end: end}
	result := make([]interval, 0, len(value.intervals)+1)
	inserted := false
	for _, current := range value.intervals {
		if current.end < merged.start {
			result = append(result, current)
			continue
		}
		if merged.end < current.start {
			if !inserted {
				result = append(result, merged)
				inserted = true
			}
			result = append(result, current)
			continue
		}
		merged.start = min(merged.start, current.start)
		merged.end = max(merged.end, current.end)
	}
	if !inserted {
		result = append(result, merged)
	}
	value.intervals = result
	return nil
}

func (p *Progress) Snapshots(terminal bool, failed bool) map[string]ProgressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[string]ProgressSnapshot, len(p.files))
	for sourceID, value := range p.files {
		var unique uint64
		for _, current := range value.intervals {
			unique += current.end - current.start
		}
		state := FileQueued
		switch {
		case unique == value.total && value.total > 0:
			state = FileFullyServed
		case failed && value.hasRequest:
			state = FileFailed
		case terminal && value.wireBytes > 0:
			state = FileInterrupted
		case value.wireBytes > 0:
			state = FileServing
		case value.hasRequest:
			state = FileRequested
		case terminal:
			state = FileNotRequested
		}
		var percent float64
		if value.total > 0 {
			percent = float64(unique) / float64(value.total) * 100
		}
		result[sourceID] = ProgressSnapshot{
			SourceID: sourceID, State: state, UniqueServedBytes: unique,
			WireBytes: value.wireBytes, TotalBytes: value.total, Percent: percent,
			RangeRequests:         value.rangeRequests,
			NonSequentialRequests: value.nonSequentialRequests,
			BackwardRequests:      value.backwardRequests,
			RepeatedRequests:      value.repeatedRequests,
			MaxRequestedOffset:    value.maxRequestedOffset,
		}
	}
	return result
}
