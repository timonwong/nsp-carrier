package main

import (
	"context"
	"time"

	appcore "github.com/timonwong/nsp-carrier/internal/app"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const snapshotEvent = "nsp-carrier:snapshot"

type DesktopApp struct {
	ctx        context.Context
	controller *appcore.Controller
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{controller: appcore.NewController()}
}

func (a *DesktopApp) startup(ctx context.Context) {
	a.ctx = ctx
	a.controller.SetSink(func(snapshot appcore.ViewSnapshot) {
		runtime.EventsEmit(ctx, snapshotEvent, snapshot)
	})
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) {
		if _, err := a.controller.Add(paths); err != nil {
			runtime.LogError(ctx, "file drop: "+err.Error())
		}
	})
}

func (a *DesktopApp) shutdown(context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.controller.Shutdown(ctx)
}

func (a *DesktopApp) beforeClose(ctx context.Context) bool {
	if !a.controller.IsBusy() {
		return false
	}
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Stop the active session?",
		Message:       "NSP Carrier is serving DBI. Stopping now will interrupt the current transfer.",
		Buttons:       []string{"Cancel", "Stop and Quit"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil || choice != "Stop and Quit" {
		return true
	}
	a.shutdown(ctx)
	return false
}

func (a *DesktopApp) GetSnapshot() appcore.ViewSnapshot {
	return a.controller.Snapshot()
}

func (a *DesktopApp) ChooseFiles() (appcore.ViewSnapshot, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Add title files",
		Filters: []runtime.FileFilter{{
			DisplayName: "Nintendo Switch title files (*.nsp, *.nsz, *.xci, *.xcz)",
			Pattern:     "*.nsp;*.nsz;*.xci;*.xcz",
		}},
		ResolvesAliases: true,
	})
	if err != nil || len(paths) == 0 {
		return a.controller.Snapshot(), err
	}
	return a.controller.Add(paths)
}

func (a *DesktopApp) ChooseFolder() (appcore.ViewSnapshot, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:           "Add a folder recursively",
		ResolvesAliases: true,
	})
	if err != nil || path == "" {
		return a.controller.Snapshot(), err
	}
	return a.controller.Add([]string{path})
}

func (a *DesktopApp) AddPaths(paths []string) (appcore.ViewSnapshot, error) {
	return a.controller.Add(paths)
}

func (a *DesktopApp) Remove(ids []string) (appcore.ViewSnapshot, error) {
	return a.controller.Remove(ids)
}

func (a *DesktopApp) ClearQueue() (appcore.ViewSnapshot, error) {
	return a.controller.Clear()
}

func (a *DesktopApp) SetSelected(id string, selected bool) (appcore.ViewSnapshot, error) {
	return a.controller.SetSelected(id, selected)
}

func (a *DesktopApp) SetAllSelected(selected bool) (appcore.ViewSnapshot, error) {
	return a.controller.SetAllSelected(selected)
}

func (a *DesktopApp) Start() (appcore.ViewSnapshot, error) {
	return a.controller.Start()
}

func (a *DesktopApp) Stop() appcore.ViewSnapshot {
	return a.controller.Stop()
}
