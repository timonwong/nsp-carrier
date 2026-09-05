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

const discoveryExampleLimit = 3

// DiscoverySkip records skipped inputs without making a mixed folder add fail.
type DiscoverySkip struct {
	Count    int
	Examples []string
}

func (s *DiscoverySkip) add(path string) {
	s.Count++
	if len(s.Examples) < discoveryExampleLimit {
		s.Examples = append(s.Examples, path)
	}
}

type DiscoveryStats struct {
	Unsupported DiscoverySkip
	Duplicate   DiscoverySkip
	Symlink     DiscoverySkip
	Hidden      DiscoverySkip
	Unreadable  DiscoverySkip
}

type DiscoveryResult struct {
	Entries []Entry
	Stats   DiscoveryStats
}

type discoveredFile struct {
	path string
	info fs.FileInfo
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
func Discover(inputs []string, supportedExtensions []string) ([]Entry, error) {
	result, err := DiscoverWithStats(inputs, supportedExtensions)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// DiscoverWithStats is the reporting variant used by queue additions. It
// filters unsupported content and continues past per-entry filesystem errors
// so one bad item cannot prevent valid files in the same folder from being added.
func DiscoverWithStats(inputs []string, supportedExtensions []string) (DiscoveryResult, error) {
	var paths []string
	seenPaths := make(map[string]struct{})
	seenFiles := make([]discoveredFile, 0)
	var stats DiscoveryStats
	allowedExtensions := make(map[string]struct{}, len(supportedExtensions))
	for _, extension := range supportedExtensions {
		allowedExtensions[strings.ToLower(extension)] = struct{}{}
	}

	addFile := func(path string, info fs.FileInfo) error {
		if strings.HasPrefix(info.Name(), ".") {
			stats.Hidden.add(path)
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			if info.Mode()&os.ModeSymlink != 0 {
				stats.Symlink.add(path)
			} else {
				stats.Unsupported.add(path)
			}
			return nil
		}
		if _, ok := allowedExtensions[strings.ToLower(filepath.Ext(path))]; !ok {
			stats.Unsupported.add(path)
			return nil
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absolute = filepath.Clean(absolute)
		if _, ok := seenPaths[absolute]; ok {
			stats.Duplicate.add(absolute)
			return nil
		}
		fileInfo, err := os.Stat(absolute)
		if err != nil {
			stats.Unreadable.add(absolute)
			return nil
		}
		for _, seen := range seenFiles {
			if strings.EqualFold(seen.path, absolute) && os.SameFile(seen.info, fileInfo) {
				stats.Duplicate.add(absolute)
				return nil
			}
		}
		seenPaths[absolute] = struct{}{}
		seenFiles = append(seenFiles, discoveredFile{path: absolute, info: fileInfo})
		paths = append(paths, absolute)
		return nil
	}

	for _, input := range inputs {
		info, err := os.Lstat(input)
		if err != nil {
			return DiscoveryResult{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			stats.Symlink.add(input)
			continue
		}
		if !info.IsDir() {
			if err := addFile(input, info); err != nil {
				return DiscoveryResult{}, err
			}
			continue
		}

		var directoryPaths []string
		err = filepath.WalkDir(input, func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				stats.Unreadable.add(path)
				if dirEntry != nil && dirEntry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if path != input && strings.HasPrefix(dirEntry.Name(), ".") {
				stats.Hidden.add(path)
				if dirEntry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if path != input && dirEntry.Type()&os.ModeSymlink != 0 {
				stats.Symlink.add(path)
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
				stats.Unreadable.add(path)
				return nil
			}
			if entryInfo.Mode().IsRegular() {
				directoryPaths = append(directoryPaths, path)
			} else {
				stats.Unsupported.add(path)
			}
			return nil
		})
		if err != nil {
			return DiscoveryResult{}, err
		}
		sort.Strings(directoryPaths)
		for _, path := range directoryPaths {
			entryInfo, err := os.Lstat(path)
			if err != nil {
				stats.Unreadable.add(path)
				continue
			}
			if err := addFile(path, entryInfo); err != nil {
				return DiscoveryResult{}, err
			}
		}
	}

	entries := make([]Entry, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			stats.Unreadable.add(path)
			continue
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
	return DiscoveryResult{Entries: entries, Stats: stats}, nil
}

func BuildCatalog(inputs []string, supportedExtensions []string) (*Catalog, error) {
	entries, err := Discover(inputs, supportedExtensions)
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

func (c *Catalog) OpenRange(sourceID string, offset uint64, size uint64) (io.ReadCloser, uint64, error) {
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
	}, available, nil
}
