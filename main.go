package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	desktop := NewDesktopApp()
	app := application.New(application.Options{
		Name:        "NSP Carrier",
		Description: "An Omni host for NS installers. Host-side completion is not proof of installation.",
		Services:    []application.Service{application.NewService(desktop)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		ShouldQuit: desktop.shouldQuit,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	desktop.app = app
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "NSP Carrier",
		Width:            1180,
		Height:           760,
		MinWidth:         860,
		MinHeight:        620,
		BackgroundColour: application.NewRGB(246, 247, 249),
		EnableFileDrop:   true,
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHiddenInset,
			Appearance: application.DefaultAppearance,
		},
	})
	desktop.window = window
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !desktop.shouldQuit() {
			e.Cancel()
		}
	})
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		if _, err := desktop.AddPaths(e.Context().DroppedFiles()); err != nil {
			app.Logger.Error("file drop: " + err.Error())
		}
	})
	if err := app.Run(); err != nil {
		println("NSP Carrier:", err.Error())
	}
}
