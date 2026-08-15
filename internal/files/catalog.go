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
	byName  map[string]Entry
}

type DuplicateBasenameError struct {
	Name  string
	Paths []string
}

func (e *DuplicateBasenameError) Error() string {
	return fmt.Sprintf("%s %q: %s", ErrDuplicateBasename, e.Name, strings.Join(e.Paths, ", "))
}

func (e *DuplicateBasenameError) Unwrap() error { return ErrDuplicateBasename }

func BuildCatalog(inputs []string) (*Catalog, error) {
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
	byName := make(map[string]Entry, len(paths))
	conflicts := make(map[string][]string)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		if previous, ok := byName[name]; ok {
			conflicts[name] = append([]string{previous.Path}, path)
			continue
		}
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
		byName[name] = entry
	}
	if len(conflicts) > 0 {
		names := make([]string, 0, len(conflicts))
		for name := range conflicts {
			names = append(names, name)
		}
		sort.Strings(names)
		name := names[0]
		return nil, &DuplicateBasenameError{Name: name, Paths: conflicts[name]}
	}

	return &Catalog{entries: entries, byName: byName}, nil
}

func (c *Catalog) Entries() []Entry {
	return append([]Entry(nil), c.entries...)
}

type sectionReadCloser struct {
	*io.SectionReader
	file *os.File
}

func (r *sectionReadCloser) Close() error { return r.file.Close() }

func (c *Catalog) OpenRange(name string, offset uint64, size uint32) (io.ReadCloser, error) {
	entry, ok := c.byName[name]
	if !ok {
		return nil, ErrFileNotFound
	}
	info, err := os.Stat(entry.Path)
	if err != nil || !os.SameFile(entry.info, info) || info.Size() != entry.Size || !info.ModTime().Equal(entry.ModTime) {
		return nil, ErrSourceChanged
	}
	if entry.Size < 0 || offset > uint64(entry.Size) || uint64(size) > uint64(entry.Size)-offset {
		return nil, ErrRangeOutOfBounds
	}

	file, err := os.Open(entry.Path)
	if err != nil {
		return nil, ErrSourceChanged
	}
	return &sectionReadCloser{
		SectionReader: io.NewSectionReader(file, int64(offset), int64(size)),
		file:          file,
	}, nil
}
