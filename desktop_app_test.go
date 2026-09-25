package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	appcore "github.com/timonwong/nsp-carrier/internal/app"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
)

func unusedRunner(context.Context, host.ProfileID, *files.Catalog, func(host.Event)) error {
	return errors.New("runner should not start")
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
