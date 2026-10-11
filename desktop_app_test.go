package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appcore "github.com/timonwong/nsp-carrier/internal/app"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

func unusedRunner(context.Context, host.ProfileID, *files.Catalog, func(host.Event)) error {
	return errors.New("runner should not start")
}

func TestContentFileFilterPreservesSupportedFormats(t *testing.T) {
	displayName, pattern := contentFileFilter()
	if want := "Nintendo Switch content files (*.nsp, *.nsz, *.xci, *.xcz, *.msp)"; displayName != want {
		t.Fatalf("display name = %q, want %q", displayName, want)
	}
	if want := "*.nsp;*.nsz;*.xci;*.xcz;*.msp"; pattern != want {
		t.Fatalf("pattern = %q, want %q", pattern, want)
	}
}

func TestSnapshotEventPreservesFrontendContract(t *testing.T) {
	if snapshotEvent != "nsp-carrier:snapshot" {
		t.Fatalf("snapshot event = %q", snapshotEvent)
	}
}

func TestChooseFolderAddsEverySelectedDirectory(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	for _, directory := range []string{left, right} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(left, "base.nsp"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "update.nsz"), []byte("update"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &DesktopApp{
		controller: appcore.NewControllerWithDependencies(unusedRunner, nil),
		openFolders: func() ([]string, error) {
			return []string{left, right}, nil
		},
	}

	snapshot, err := app.ChooseFolder()
	if err != nil {
		t.Fatalf("ChooseFolder() error = %v", err)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("items = %#v", snapshot.Items)
	}
	names := map[string]bool{}
	for _, item := range snapshot.Items {
		names[item.Name] = true
	}
	if !names["base.nsp"] || !names["update.nsz"] {
		t.Fatalf("items = %#v", snapshot.Items)
	}
}

func TestChooseFolderKeepsQueueUnchangedOnCancel(t *testing.T) {
	app := &DesktopApp{
		controller: appcore.NewControllerWithDependencies(unusedRunner, nil),
		openFolders: func() ([]string, error) {
			return nil, nil
		},
	}

	before := app.GetSnapshot()
	snapshot, err := app.ChooseFolder()
	if err != nil {
		t.Fatalf("ChooseFolder() error = %v", err)
	}
	if len(snapshot.Items) != 0 || len(snapshot.Logs) != len(before.Logs) {
		t.Fatalf("cancelled snapshot = %#v", snapshot)
	}
}

// Exit can fail by bypassing confirmation, opening duplicate dialogs, quitting
// before the runner drains, or stopping twice through window and app hooks.
func TestBusyCloseCancelKeepsSessionAndAllowsAnotherRequest(t *testing.T) {
	app, _ := busyDesktopApp(t)
	var cancel func()
	prompts := 0
	app.confirmQuit = func(_ func(), onCancel func()) {
		prompts++
		cancel = onCancel
	}
	if app.shouldQuit() || app.shouldQuit() || prompts != 1 {
		t.Fatalf("busy close must be held by one confirmation; prompts = %d", prompts)
	}
	cancel()
	if !app.controller.IsBusy() {
		t.Fatal("Cancel stopped the serving session")
	}
	if app.shouldQuit() || prompts != 2 {
		t.Fatalf("cancelled close must allow another confirmation; prompts = %d", prompts)
	}
}

func TestBusyCloseConfirmDrainsSessionBeforeQuit(t *testing.T) {
	app, runnerDone := busyDesktopApp(t)
	var confirm func()
	app.confirmQuit = func(onConfirm func(), _ func()) { confirm = onConfirm }
	quit := make(chan struct{})
	app.quit = func() {
		select {
		case <-runnerDone:
		default:
			t.Error("quit happened before session shutdown")
		}
		close(quit)
	}
	if app.shouldQuit() {
		t.Fatal("busy close bypassed confirmation")
	}
	confirm()
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("confirmed close did not quit")
	}
	if !app.shouldQuit() {
		t.Fatal("confirmed close asked again on app quit")
	}
	if err := app.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
}

func TestIdleCloseDoesNotAsk(t *testing.T) {
	app := NewDesktopApp()
	app.confirmQuit = func(func(), func()) { t.Fatal("idle close asked for confirmation") }
	if !app.shouldQuit() {
		t.Fatal("idle close was cancelled")
	}
	if err := app.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
}

func busyDesktopApp(t *testing.T) (*DesktopApp, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	app := &DesktopApp{controller: appcore.NewControllerWithDependencies(
		func(ctx context.Context, _ host.ProfileID, _ *files.Catalog, _ func(host.Event)) error {
			<-ctx.Done()
			close(done)
			return ctx.Err()
		}, nil,
	)}
	path := filepath.Join(t.TempDir(), "title.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AddPaths([]string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ServiceShutdown() })
	return app, done
}

func TestServiceShutdownStopsSessionOnceAcrossConcurrentClosePaths(t *testing.T) {
	cancelled, release := make(chan struct{}), make(chan struct{})
	app := &DesktopApp{controller: appcore.NewControllerWithDependencies(
		func(ctx context.Context, _ host.ProfileID, _ *files.Catalog, _ func(host.Event)) error {
			<-ctx.Done()
			close(cancelled)
			<-release
			return ctx.Err()
		}, nil,
	)}
	path := filepath.Join(t.TempDir(), "title.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AddPaths([]string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start(); err != nil {
		t.Fatal(err)
	}
	var stopping atomic.Int32
	app.controller.SetSink(func(snapshot appcore.ViewSnapshot) {
		if snapshot.State == appcore.StateStopping {
			stopping.Add(1)
		}
	})
	var calls sync.WaitGroup
	for range 2 {
		calls.Go(func() { _ = app.ServiceShutdown() })
	}
	<-cancelled
	close(release)
	calls.Wait()
	if got := stopping.Load(); got != 1 {
		t.Fatalf("shutdown emitted %d Stop snapshots, want one", got)
	}
}
