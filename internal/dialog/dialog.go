package dialog

import "errors"

const FolderDialogTitle = "Add folders recursively"

var errSingleFolderFallback = errors.New("multi-folder picker unavailable")

func IsSingleFolderFallback(err error) bool {
	return errors.Is(err, errSingleFolderFallback)
}

type folderPanel struct {
	CanChooseFiles             bool
	CanChooseDirectories       bool
	AllowsMultipleSelection    bool
	ResolvesAliases            bool
	CanCreateDirectories       bool
	TreatPackagesAsDirectories bool
}

func folderPanelConfig() folderPanel {
	return folderPanel{
		CanChooseDirectories:    true,
		AllowsMultipleSelection: true,
		ResolvesAliases:         true,
	}
}

func folderPickerOptions(existing uint32) uint32 {
	const (
		fosPickFolders      = 0x20
		fosForceFilesystem  = 0x40
		fosAllowMultiselect = 0x200
		fosPathMustExist    = 0x800
	)
	return existing | fosPickFolders | fosForceFilesystem | fosAllowMultiselect | fosPathMustExist
}

func normalizeSelection(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	var selected []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		selected = append(selected, path)
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}
