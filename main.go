package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	desktop := NewDesktopApp()
	if err := wails.Run(&options.App{
		Title:     "NSP Carrier",
		Width:     1180,
		Height:    760,
		MinWidth:  860,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: options.NewRGB(246, 247, 249),
		OnStartup:        desktop.startup,
		OnShutdown:       desktop.shutdown,
		OnBeforeClose:    desktop.beforeClose,
		Bind: []interface{}{
			desktop,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.DefaultAppearance,
			About: &mac.AboutInfo{
				Title:   "NSP Carrier",
				Message: "An Omni host for NS installers. Host-side completion is not proof of installation.",
			},
		},
	}); err != nil {
		println("NSP Carrier:", err.Error())
	}
}
