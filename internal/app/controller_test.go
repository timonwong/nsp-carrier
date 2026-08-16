package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

func TestControllerQueueSnapshotsEncodeEmptyCollectionsAsArrays(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.nsp")
	second := filepath.Join(root, "second.nsp")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	controller := newController(func(context.Context, host.ProfileID, *files.Catalog, func(host.Event)) error {
		return errors.New("runner should not start")
	})
	initial := controller.Snapshot()
	if initial.Items == nil || initial.Logs == nil {
		t.Fatalf("initial snapshot collections must be arrays: %#v", initial)
	}
	snapshot, err := controller.Add([]string{first, second})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	snapshot, err = controller.Remove([]string{snapshot.Items[0].ID})
	if err != nil {
		t.Fatalf("Remove(first) error = %v", err)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Name != "second.nsp" {
		t.Fatalf("snapshot after removing first item = %#v", snapshot.Items)
	}

	snapshot, err = controller.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) == "" || snapshot.Items == nil || snapshot.Logs == nil {
		t.Fatalf("empty snapshot collections must encode as arrays: %s", encoded)
	}
	var wailsContract struct {
		ActiveProfile    host.ProfileID             `json:"activeProfile"`
		Profiles         []host.Profile             `json:"profiles"`
		ValidationErrors []host.ItemValidationError `json:"validationErrors"`
	}
	if err := json.Unmarshal(encoded, &wailsContract); err != nil {
		t.Fatal(err)
	}
	if wailsContract.ActiveProfile != host.ProfileDBI || !slices.EqualFunc(wailsContract.Profiles, host.Profiles(), func(left, right host.Profile) bool {
		return reflect.DeepEqual(left, right)
	}) || wailsContract.ValidationErrors == nil {
		t.Fatalf("Wails profile contract = %#v", wailsContract)
	}
}

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

	controller := newController(func(context.Context, host.ProfileID, *files.Catalog, func(host.Event)) error {
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
	controller := newController(func(ctx context.Context, _ host.ProfileID, _ *files.Catalog, update func(host.Event)) error {
		update(host.Event{State: host.StateConnected, SessionID: "session-1"})
		update(host.Event{State: host.StateServing, SessionID: "session-1"})
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

func TestControllerPersistsIdleOnlyProfileAndRevalidatesWithoutMutatingSelection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"base.nsp", "compressed.nsz"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store := &fakeProfileStore{profile: host.ProfileDBI}
	runnerStarted := make(chan host.ProfileID, 1)
	releaseRunner := make(chan struct{})
	controller := NewControllerWithDependencies(func(ctx context.Context, profile host.ProfileID, _ *files.Catalog, _ func(host.Event)) error {
		runnerStarted <- profile
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseRunner:
			return nil
		}
	}, store)
	snapshot, err := controller.Add([]string{root})
	if err != nil || !snapshot.CanStart || snapshot.ActiveProfile != host.ProfileDBI {
		t.Fatalf("DBI snapshot = %#v, %v", snapshot, err)
	}
	beforeSelection := []bool{snapshot.Items[0].Selected, snapshot.Items[1].Selected}
	snapshot, err = controller.SetProfile(string(host.ProfileGoldleaf))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ActiveProfile != host.ProfileGoldleaf || snapshot.CanStart || len(snapshot.ValidationErrors) != 1 || store.profile != host.ProfileGoldleaf {
		t.Fatalf("Goldleaf snapshot = %#v, stored = %q", snapshot, store.profile)
	}
	afterSelection := []bool{snapshot.Items[0].Selected, snapshot.Items[1].Selected}
	if !slices.Equal(beforeSelection, afterSelection) {
		t.Fatalf("profile change mutated selection: %v -> %v", beforeSelection, afterSelection)
	}
	var compressedID string
	for _, item := range snapshot.Items {
		if item.Name == "compressed.nsz" {
			compressedID = item.ID
		}
	}
	snapshot, err = controller.SetSelected(compressedID, false)
	if err != nil || snapshot.CanStart || len(snapshot.ValidationErrors) != 0 {
		t.Fatalf("revalidated snapshot = %#v, %v", snapshot, err)
	}
	snapshot, err = controller.SetProfile(string(host.ProfileDBI))
	if err != nil || !snapshot.CanStart {
		t.Fatalf("DBI restored snapshot = %#v, %v", snapshot, err)
	}
	if _, err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	if profile := <-runnerStarted; profile != host.ProfileDBI {
		t.Fatalf("runner profile = %q", profile)
	}
	if _, err := controller.SetProfile(string(host.ProfileDBI)); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy SetProfile() error = %v", err)
	}
	close(releaseRunner)
}

type fakeProfileStore struct{ profile host.ProfileID }

func (s *fakeProfileStore) LoadProfile() (host.ProfileID, error) { return s.profile, nil }
func (s *fakeProfileStore) SaveProfile(profile host.ProfileID) error {
	s.profile = profile
	return nil
}
