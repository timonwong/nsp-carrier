package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/timonwong/nsp-carrier/internal/files"
)

func TestControllerKeepsConflictsVisibleAndResolvesThemBySelection(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	for _, directory := range []string{left, right} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "game.nsp"), []byte(directory), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	controller := newController(func(context.Context, *files.Catalog, func(sessionUpdate)) error {
		return errors.New("runner should not start")
	})
	snapshot, err := controller.Add([]string{left, right})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if len(snapshot.Items) != 2 || !snapshot.HasConflict || snapshot.CanStart {
		t.Fatalf("conflicting snapshot = %#v", snapshot)
	}

	snapshot, err = controller.SetSelected(snapshot.Items[1].ID, false)
	if err != nil {
		t.Fatalf("SetSelected() error = %v", err)
	}
	if snapshot.HasConflict || !snapshot.CanStart || snapshot.SelectedCount != 1 {
		t.Fatalf("resolved snapshot = %#v", snapshot)
	}
}

func TestControllerOwnsStartStopLifecycleAcrossItsInterface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	runnerStarted := make(chan struct{})
	controller := newController(func(ctx context.Context, _ *files.Catalog, update func(sessionUpdate)) error {
		update(sessionUpdate{state: StateConnected, sessionID: "session-1"})
		update(sessionUpdate{state: StateServing, sessionID: "session-1"})
		close(runnerStarted)
		<-ctx.Done()
		return ctx.Err()
	})
	if _, err := controller.Add([]string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-runnerStarted:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	if snapshot := controller.Snapshot(); snapshot.State != StateServing || snapshot.SessionID != "session-1" {
		t.Fatalf("serving snapshot = %#v", snapshot)
	}
	if _, err := controller.Clear(); !errors.Is(err, ErrBusy) {
		t.Fatalf("Clear() error = %v, want ErrBusy", err)
	}

	controller.Stop()
	deadline := time.Now().Add(time.Second)
	for controller.Snapshot().State != StateIdle && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if snapshot := controller.Snapshot(); snapshot.State != StateIdle || snapshot.CanStop {
		t.Fatalf("stopped snapshot = %#v", snapshot)
	}
}
