//go:build !darwin && !windows

package dialog

func OpenFolders() ([]string, error) {
	return nil, errSingleFolderFallback
}
