package dbi

import (
	"errors"
	"sync"
)

var ErrProgressRangeOutOfBounds = errors.New("progress range out of bounds")

type interval struct {
	start uint64
	end   uint64
}

type Progress struct {
	mu        sync.Mutex
	total     uint64
	intervals []interval
	wireBytes uint64
}

type ProgressSnapshot struct {
	UniqueServedBytes uint64
	WireBytes         uint64
	TotalBytes        uint64
	Percent           float64
}

func NewProgress(total uint64) *Progress {
	return &Progress{total: total}
}

func (p *Progress) Record(offset uint64, size uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	end := offset + uint64(size)
	if end < offset || offset > p.total || end > p.total {
		return ErrProgressRangeOutOfBounds
	}
	p.wireBytes += uint64(size)

	merged := interval{start: offset, end: end}
	result := make([]interval, 0, len(p.intervals)+1)
	inserted := false
	for _, current := range p.intervals {
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
		if current.start < merged.start {
			merged.start = current.start
		}
		if current.end > merged.end {
			merged.end = current.end
		}
	}
	if !inserted {
		result = append(result, merged)
	}
	p.intervals = result
	return nil
}

func (p *Progress) Snapshot() ProgressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	var unique uint64
	for _, current := range p.intervals {
		unique += current.end - current.start
	}
	var percent float64
	if p.total > 0 {
		percent = float64(unique) / float64(p.total) * 100
	}
	return ProgressSnapshot{
		UniqueServedBytes: unique,
		WireBytes:         p.wireBytes,
		TotalBytes:        p.total,
		Percent:           percent,
	}
}
