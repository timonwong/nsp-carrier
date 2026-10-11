package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appcore "github.com/timonwong/nsp-carrier/internal/app"
	"github.com/timonwong/nsp-carrier/internal/dialog"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const snapshotEvent = "nsp-carrier:snapshot"

func init() {
	application.RegisterEvent[appcore.ViewSnapshot](snapshotEvent)
}

type DesktopApp struct {
	app          *application.App
	window       application.Window
	controller   *appcore.Controller
	openFolders  func() ([]string, error)
	confirmQuit  func(onConfirm, onCancel func())
	quit         func()
	closeMu      sync.Mutex
	confirming   bool
	closing      bool
	shutdownOnce sync.Once
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{controller: appcore.NewController()}
}

func (a *DesktopApp) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.controller.SetSink(func(snapshot appcore.ViewSnapshot) {
		a.app.Event.Emit(snapshotEvent, snapshot)
	})
	return nil
}

func (a *DesktopApp) ServiceShutdown() error {
	a.shutdownOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.controller.Shutdown(ctx)
	})
	return nil
}

func (a *DesktopApp) shouldQuit() bool {
	a.closeMu.Lock()
	if a.closing {
		a.closeMu.Unlock()
		return true
	}
	if a.confirming {
		a.closeMu.Unlock()
		return false
	}
	if !a.controller.IsBusy() {
		a.closing = true
		a.closeMu.Unlock()
		_ = a.ServiceShutdown()
		return true
	}
	a.confirming = true
	a.closeMu.Unlock()

	confirm := a.confirmQuit
	if confirm == nil {
		confirm = a.showQuitConfirmation
	}
	// beta.28 shows macOS questions asynchronously. Hold both close paths
	// until the callback has drained the session, then request app termination.
	confirm(func() {
		go func() {
			_ = a.ServiceShutdown()
			a.closeMu.Lock()
			a.closing = true
			a.closeMu.Unlock()
			if a.quit != nil {
				a.quit()
			} else {
				a.app.Quit()
			}
		}()
	}, func() {
		a.closeMu.Lock()
		a.confirming = false
		a.closeMu.Unlock()
	})
	return false
}

func (a *DesktopApp) showQuitConfirmation(onConfirm, onCancel func()) {
	question := a.app.Dialog.Question().
		SetTitle("Stop the active session?").
		SetMessage("NSP Carrier is serving the selected installer profile. Stopping now will interrupt the current transfer.").
		AttachToWindow(a.window)
	question.AddButton("Cancel").SetAsDefault().SetAsCancel().OnClick(onCancel)
	question.AddButton("Stop and Quit").OnClick(onConfirm)
	question.Show()
}

func (a *DesktopApp) GetSnapshot() appcore.ViewSnapshot {
	return a.controller.Snapshot()
}

func (a *DesktopApp) ChooseFiles() (appcore.ViewSnapshot, error) {
	displayName, pattern := contentFileFilter()
	paths, err := a.app.Dialog.OpenFile().
		SetTitle("Add title files").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		ResolvesAliases(true).
		AttachToWindow(a.window).
		AddFilter(displayName, pattern).
		PromptForMultipleSelection()
	if err != nil || len(paths) == 0 {
		return a.controller.Snapshot(), err
	}
	return a.controller.Add(paths)
}

func contentFileFilter() (string, string) {
	extensions := host.AllSupportedExtensions()
	patterns := make([]string, len(extensions))
	for index, extension := range extensions {
		patterns[index] = "*" + extension
	}
	return fmt.Sprintf("Nintendo Switch content files (%s)", strings.Join(patterns, ", ")), strings.Join(patterns, ";")
}

func (a *DesktopApp) ChooseFolder() (appcore.ViewSnapshot, error) {
	openFolders := a.openFolders
	if openFolders == nil {
		openFolders = dialog.OpenFolders
	}
	paths, err := openFolders()
	if dialog.IsSingleFolderFallback(err) {
		path, err := a.app.Dialog.OpenFile().
			SetTitle(dialog.FolderDialogTitle).
			CanChooseDirectories(true).
			CanChooseFiles(false).
			ResolvesAliases(true).
			AttachToWindow(a.window).
			PromptForSingleSelection()
		if err != nil || path == "" {
			return a.controller.Snapshot(), err
		}
		paths = []string{path}
	} else if err != nil || len(paths) == 0 {
		return a.controller.Snapshot(), err
	}
	return a.controller.Add(paths)
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

func (a *DesktopApp) SetProfile(profileID string) (appcore.ViewSnapshot, error) {
	return a.controller.SetProfile(profileID)
}

func (a *DesktopApp) Start() (appcore.ViewSnapshot, error) {
	return a.controller.Start()
}

func (a *DesktopApp) Stop() appcore.ViewSnapshot {
	return a.controller.Stop()
}
