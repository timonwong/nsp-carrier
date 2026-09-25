package dialog

import "testing"

func TestFolderPanelConfigSelectsDirectoriesOnlyWithMultipleSelection(t *testing.T) {
	config := folderPanelConfig()
	if config.CanChooseFiles || !config.CanChooseDirectories || !config.AllowsMultipleSelection {
		t.Fatalf("folder panel config = %#v", config)
	}
	if !config.ResolvesAliases || config.CanCreateDirectories {
		t.Fatalf("folder panel config = %#v", config)
	}
}

func TestFolderPickerOptionsEnablePickFoldersAndMultiSelect(t *testing.T) {
	const (
		fosPickFolders      = 0x20
		fosForceFilesystem  = 0x40
		fosAllowMultiselect = 0x200
		fosPathMustExist    = 0x800
		fosFileMustExist    = 0x1000
	)
	got := folderPickerOptions(fosFileMustExist | fosPathMustExist)
	if got&fosPickFolders == 0 || got&fosAllowMultiselect == 0 || got&fosForceFilesystem == 0 || got&fosPathMustExist == 0 {
		t.Fatalf("folder picker options = %#x", got)
	}
}

func TestNormalizeFolderSelectionTreatsCancelAndEmptyAsNoPaths(t *testing.T) {
	if paths := normalizeSelection(nil); paths != nil {
		t.Fatalf("nil selection = %#v", paths)
	}
	if paths := normalizeSelection([]string{}); paths != nil {
		t.Fatalf("empty selection = %#v", paths)
	}
	if paths := normalizeSelection([]string{"", ""}); paths != nil {
		t.Fatalf("blank selection = %#v", paths)
	}
	got := normalizeSelection([]string{"/Games", "", "/DLC"})
	if len(got) != 2 || got[0] != "/Games" || got[1] != "/DLC" {
		t.Fatalf("normalized selection = %#v", got)
	}
}

func TestFolderDialogTitleNamesMultiSelect(t *testing.T) {
	if FolderDialogTitle != "Add folders recursively" {
		t.Fatalf("FolderDialogTitle = %q", FolderDialogTitle)
	}
}
