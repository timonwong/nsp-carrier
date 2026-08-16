package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

var (
	ErrBusy          = errors.New("a USB session is active")
	ErrEmptyQueue    = errors.New("select at least one file")
	ErrQueueConflict = errors.New("selected files contain duplicate basenames")
	ErrQueueItem     = errors.New("queue item not found")
)

const maxLogEntries = 300

type QueueItem struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Path              string  `json:"path"`
	Size              int64   `json:"size"`
	Selected          bool    `json:"selected"`
	Conflict          bool    `json:"conflict"`
	Status            string  `json:"status"`
	UniqueServedBytes uint64  `json:"uniqueServedBytes"`
	WireBytes         uint64  `json:"wireBytes"`
	Progress          float64 `json:"progress"`
	Requested         bool    `json:"requested"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type ViewSnapshot struct {
	State             State       `json:"state"`
	SessionID         string      `json:"sessionId"`
	Items             []QueueItem `json:"items"`
	Logs              []LogEntry  `json:"logs"`
	SelectedCount     int         `json:"selectedCount"`
	SelectedBytes     int64       `json:"selectedBytes"`
	RequestedBytes    uint64      `json:"requestedBytes"`
	UniqueServedBytes uint64      `json:"uniqueServedBytes"`
	WireBytes         uint64      `json:"wireBytes"`
	OverallProgress   float64     `json:"overallProgress"`
	CanStart          bool        `json:"canStart"`
	CanStop           bool        `json:"canStop"`
	HasConflict       bool        `json:"hasConflict"`
	LastError         string      `json:"lastError"`
}

type sessionUpdate struct {
	state     State
	sessionID string
	progress  map[string]host.ProgressSnapshot
	err       error
}

type sessionRunner func(context.Context, *files.Catalog, func(sessionUpdate)) error

type Controller struct {
	mu        sync.Mutex
	items     []QueueItem
	logs      []LogEntry
	state     State
	sessionID string
	lastError string
	sink      func(ViewSnapshot)
	runner    sessionRunner
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewController() *Controller {
	return newController(runUSBSession)
}

func newController(runner sessionRunner) *Controller {
	return &Controller{state: StateIdle, runner: runner}
}

func (c *Controller) SetSink(sink func(ViewSnapshot)) {
	c.mu.Lock()
	c.sink = sink
	snapshot := c.snapshotLocked()
	c.mu.Unlock()
	if sink != nil {
		sink(snapshot)
	}
}

func (c *Controller) Snapshot() ViewSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked()
}

func (c *Controller) Add(inputs []string) (ViewSnapshot, error) {
	discovered, err := files.Discover(inputs)
	if err != nil {
		return c.Snapshot(), err
	}

	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	existing := make(map[string]struct{}, len(c.items))
	for _, item := range c.items {
		existing[item.Path] = struct{}{}
	}
	added := 0
	for _, entry := range discovered {
		if _, exists := existing[entry.Path]; exists {
			continue
		}
		c.items = append(c.items, QueueItem{
			ID: entry.ID, Name: entry.Name, Path: entry.Path, Size: entry.Size,
			Selected: true, Status: "Queued",
		})
		existing[entry.Path] = struct{}{}
		added++
	}
	c.recomputeConflictsLocked()
	c.appendLogLocked("info", fmt.Sprintf("Added %d supported file(s)", added))
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot, nil
}

func (c *Controller) Remove(ids []string) (ViewSnapshot, error) {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	remove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	before := len(c.items)
	c.items = slices.DeleteFunc(c.items, func(item QueueItem) bool {
		_, exists := remove[item.ID]
		return exists
	})
	removed := before - len(c.items)
	if removed == 0 && len(ids) > 0 {
		c.mu.Unlock()
		return c.Snapshot(), ErrQueueItem
	}
	c.recomputeConflictsLocked()
	c.appendLogLocked("info", fmt.Sprintf("Removed %d file(s)", removed))
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot, nil
}

func (c *Controller) Clear() (ViewSnapshot, error) {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	c.items = nil
	c.appendLogLocked("info", "Cleared the queue")
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot, nil
}

func (c *Controller) SetSelected(id string, selected bool) (ViewSnapshot, error) {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	found := false
	for index := range c.items {
		if c.items[index].ID == id {
			c.items[index].Selected = selected
			found = true
			break
		}
	}
	if !found {
		c.mu.Unlock()
		return c.Snapshot(), ErrQueueItem
	}
	c.recomputeConflictsLocked()
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot, nil
}

func (c *Controller) SetAllSelected(selected bool) (ViewSnapshot, error) {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	for index := range c.items {
		c.items[index].Selected = selected
	}
	c.recomputeConflictsLocked()
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot, nil
}

func (c *Controller) Start() (ViewSnapshot, error) {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return c.Snapshot(), ErrBusy
	}
	var paths []string
	for _, item := range c.items {
		if item.Selected {
			paths = append(paths, item.Path)
		}
	}
	if len(paths) == 0 {
		c.mu.Unlock()
		return c.Snapshot(), ErrEmptyQueue
	}
	if c.hasConflictLocked() {
		c.mu.Unlock()
		return c.Snapshot(), ErrQueueConflict
	}
	catalog, err := files.BuildCatalog(paths)
	if err != nil {
		c.mu.Unlock()
		return c.Snapshot(), err
	}
	for index := range c.items {
		c.items[index].Status = "Queued"
		c.items[index].UniqueServedBytes = 0
		c.items[index].WireBytes = 0
		c.items[index].Progress = 0
		c.items[index].Requested = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.done = make(chan struct{})
	c.state = StateWaitingForDevice
	c.sessionID = ""
	c.lastError = ""
	c.appendLogLocked("info", fmt.Sprintf("Waiting for DBI with %d selected file(s)", len(paths)))
	snapshot, sink, done := c.snapshotLocked(), c.sink, c.done
	c.mu.Unlock()
	c.emit(sink, snapshot)

	go func() {
		err := c.runner(ctx, catalog, c.handleSessionUpdate)
		c.finishSession(err)
		close(done)
	}()
	return snapshot, nil
}

func (c *Controller) Stop() ViewSnapshot {
	c.mu.Lock()
	if c.state == StateIdle {
		snapshot := c.snapshotLocked()
		c.mu.Unlock()
		return snapshot
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.state = StateStopping
	c.appendLogLocked("info", "Stopping USB session")
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
	return snapshot
}

func (c *Controller) Shutdown(ctx context.Context) {
	c.Stop()
	c.mu.Lock()
	done := c.done
	c.mu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (c *Controller) IsBusy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state != StateIdle
}

func (c *Controller) handleSessionUpdate(update sessionUpdate) {
	c.mu.Lock()
	previousState := c.state
	if previousState == StateStopping && update.state != StateFailed && update.state != StateCompleted {
		update.state = StateStopping
	}
	c.state = update.state
	if update.sessionID != "" {
		c.sessionID = update.sessionID
	}
	if update.err != nil {
		c.lastError = update.err.Error()
		c.appendLogLocked("error", update.err.Error())
	}
	c.applyProgressLocked(update.progress)
	if update.state == StateConnected && previousState != StateConnected {
		c.appendLogLocked("info", "Nintendo Switch connected")
	} else if update.state == StateServing && previousState != StateServing {
		c.appendLogLocked("info", "Serving DBI file requests")
	} else if update.state == StateDisconnected && previousState != StateDisconnected {
		c.appendLogLocked("warning", "Nintendo Switch disconnected; waiting for a fresh session")
	} else if update.state == StateCompleted && previousState != StateCompleted {
		c.appendLogLocked("success", "Host-side DBI session completed")
	}
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
}

func (c *Controller) finishSession(err error) {
	c.mu.Lock()
	if err != nil && !errors.Is(err, context.Canceled) {
		c.lastError = err.Error()
		c.appendLogLocked("error", err.Error())
	}
	if c.state != StateCompleted && c.state != StateFailed {
		c.state = StateStopping
	}
	c.cancel = nil
	c.state = StateIdle
	c.sessionID = ""
	c.appendLogLocked("info", "USB session is idle")
	snapshot, sink := c.snapshotLocked(), c.sink
	c.mu.Unlock()
	c.emit(sink, snapshot)
}

func (c *Controller) applyProgressLocked(progress map[string]host.ProgressSnapshot) {
	for index := range c.items {
		item := &c.items[index]
		value, exists := progress[item.ID]
		if !exists {
			continue
		}
		item.UniqueServedBytes = value.UniqueServedBytes
		item.WireBytes = value.WireBytes
		item.Progress = value.Percent
		item.Requested = value.RangeRequests > 0
		if value.State != "" {
			item.Status = string(value.State)
		}
	}
}

func (c *Controller) recomputeConflictsLocked() {
	counts := make(map[string]int)
	for _, item := range c.items {
		if item.Selected {
			counts[item.Name]++
		}
	}
	for index := range c.items {
		c.items[index].Conflict = c.items[index].Selected && counts[c.items[index].Name] > 1
	}
}

func (c *Controller) hasConflictLocked() bool {
	for _, item := range c.items {
		if item.Conflict {
			return true
		}
	}
	return false
}

func (c *Controller) appendLogLocked(level, message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	c.logs = append(c.logs, LogEntry{Time: time.Now().Format("15:04:05"), Level: level, Message: message})
	if len(c.logs) > maxLogEntries {
		c.logs = append([]LogEntry(nil), c.logs[len(c.logs)-maxLogEntries:]...)
	}
}

func (c *Controller) snapshotLocked() ViewSnapshot {
	snapshot := ViewSnapshot{
		State: c.state, SessionID: c.sessionID, LastError: c.lastError,
		Items:   append([]QueueItem{}, c.items...),
		Logs:    append([]LogEntry{}, c.logs...),
		CanStop: c.state != StateIdle,
	}
	for _, item := range c.items {
		if item.Selected {
			snapshot.SelectedCount++
			snapshot.SelectedBytes += item.Size
		}
		if item.Requested {
			snapshot.RequestedBytes += uint64(item.Size)
		}
		snapshot.UniqueServedBytes += item.UniqueServedBytes
		snapshot.WireBytes += item.WireBytes
		if item.Conflict {
			snapshot.HasConflict = true
		}
	}
	if snapshot.RequestedBytes > 0 {
		snapshot.OverallProgress = float64(snapshot.UniqueServedBytes) / float64(snapshot.RequestedBytes) * 100
	}
	snapshot.CanStart = c.state == StateIdle && snapshot.SelectedCount > 0 && !snapshot.HasConflict
	return snapshot
}

func (c *Controller) emit(sink func(ViewSnapshot), snapshot ViewSnapshot) {
	if sink != nil {
		sink(snapshot)
	}
}
