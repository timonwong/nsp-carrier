package files

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrDuplicateBasename = errors.New("duplicate basename")
	ErrFileNotFound      = errors.New("file not found in catalog")
	ErrRangeOutOfBounds  = errors.New("file range out of bounds")
	ErrSourceChanged     = errors.New("source file changed")
)

var supportedExtensions = map[string]struct{}{
	".nsp": {},
	".nsz": {},
	".xci": {},
	".xcz": {},
}

type Entry struct {
	ID      string
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
	info    fs.FileInfo
}

type Catalog struct {
	entries []Entry
	byID    map[string]Entry
}

type DuplicateBasenameError struct {
	Name  string
	Paths []string
}

func (e *DuplicateBasenameError) Error() string {
	return fmt.Sprintf("%s %q: %s", ErrDuplicateBasename, e.Name, strings.Join(e.Paths, ", "))
}

func (e *DuplicateBasenameError) Unwrap() error { return ErrDuplicateBasename }

// Discover expands files and directories into supported regular files while
// preserving addition order. It intentionally keeps duplicate basenames so a
// queue UI can present and resolve those conflicts before freezing a Catalog.
func Discover(inputs []string) ([]Entry, error) {
	var paths []string
	seenPaths := make(map[string]struct{})

	addFile := func(path string, info fs.FileInfo) error {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		if _, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]; !ok {
			return nil
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absolute = filepath.Clean(absolute)
		if _, ok := seenPaths[absolute]; ok {
			return nil
		}
		seenPaths[absolute] = struct{}{}
		paths = append(paths, absolute)
		return nil
	}

	for _, input := range inputs {
		info, err := os.Lstat(input)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !info.IsDir() {
			if err := addFile(input, info); err != nil {
				return nil, err
			}
			continue
		}

		var directoryPaths []string
		err = filepath.WalkDir(input, func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path != input && dirEntry.Type()&os.ModeSymlink != 0 {
				if dirEntry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if dirEntry.IsDir() {
				return nil
			}
			entryInfo, err := dirEntry.Info()
			if err != nil {
				return err
			}
			if entryInfo.Mode().IsRegular() {
				directoryPaths = append(directoryPaths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(directoryPaths)
		for _, path := range directoryPaths {
			entryInfo, err := os.Lstat(path)
			if err != nil {
				return nil, err
			}
			if err := addFile(path, entryInfo); err != nil {
				return nil, err
			}
		}
	}

	entries := make([]Entry, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		hash := sha256.Sum256([]byte(path))
		entry := Entry{
			ID:      hex.EncodeToString(hash[:16]),
			Path:    path,
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			info:    info,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func BuildCatalog(inputs []string) (*Catalog, error) {
	entries, err := Discover(inputs)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}

	return &Catalog{entries: entries, byID: byID}, nil
}

func (c *Catalog) Entries() []Entry {
	return append([]Entry(nil), c.entries...)
}

type sectionReadCloser struct {
	*io.SectionReader
	file *os.File
}

func (r *sectionReadCloser) Close() error { return r.file.Close() }

func (c *Catalog) OpenRange(sourceID string, offset uint64, size uint32) (io.ReadCloser, uint32, error) {
	entry, ok := c.byID[sourceID]
	if !ok {
		return nil, 0, ErrFileNotFound
	}
	info, err := os.Stat(entry.Path)
	if err != nil || !os.SameFile(entry.info, info) || info.Size() != entry.Size || !info.ModTime().Equal(entry.ModTime) {
		return nil, 0, ErrSourceChanged
	}
	if entry.Size < 0 || offset > uint64(entry.Size) {
		return nil, 0, ErrRangeOutOfBounds
	}
	available := min(uint64(size), uint64(entry.Size)-offset)

	file, err := os.Open(entry.Path)
	if err != nil {
		return nil, 0, ErrSourceChanged
	}
	return &sectionReadCloser{
		SectionReader: io.NewSectionReader(file, int64(offset), int64(available)),
		file:          file,
	}, uint32(available), nil
}
