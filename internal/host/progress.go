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
	requestedIntervals    []interval
	servedIntervals       []interval
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
	mu                    sync.Mutex
	files                 map[string]*fileProgress
	completeWhenRequested bool
}

func NewProgress(catalog *files.Catalog, profile ProfileID) *Progress {
	tracked := make(map[string]*fileProgress, len(catalog.Entries()))
	for _, entry := range catalog.Entries() {
		tracked[entry.ID] = &fileProgress{total: uint64(entry.Size)}
	}
	return &Progress{files: tracked, completeWhenRequested: profile == ProfileAwoo}
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
	end := value.lastRequestEnd
	if offset < value.total {
		end = min(end, value.total)
		value.requestedIntervals = mergeInterval(value.requestedIntervals, interval{start: offset, end: end})
	}
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
	value.servedIntervals = mergeInterval(value.servedIntervals, interval{start: offset, end: end})
	return nil
}

func mergeInterval(intervals []interval, merged interval) []interval {
	if merged.start >= merged.end {
		return intervals
	}
	result := make([]interval, 0, len(intervals)+1)
	inserted := false
	for _, current := range intervals {
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
	return result
}

func intervalsCover(covering, covered []interval) bool {
	coveringIndex := 0
	for _, target := range covered {
		for coveringIndex < len(covering) && covering[coveringIndex].end < target.end {
			coveringIndex++
		}
		if coveringIndex >= len(covering) || covering[coveringIndex].start > target.start || covering[coveringIndex].end < target.end {
			return false
		}
	}
	return true
}

func (p *Progress) Snapshots(terminal bool, failed bool) map[string]ProgressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[string]ProgressSnapshot, len(p.files))
	for sourceID, value := range p.files {
		var unique uint64
		for _, current := range value.servedIntervals {
			unique += current.end - current.start
		}
		fullyServed := unique == value.total && value.total > 0
		if p.completeWhenRequested {
			fullyServed = value.hasRequest && intervalsCover(value.servedIntervals, value.requestedIntervals)
		}
		state := FileQueued
		switch {
		case fullyServed:
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
