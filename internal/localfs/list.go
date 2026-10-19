package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry is one item in a local directory listing.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
}

// Listing is a cleaned local directory and its entries.
type Listing struct {
	Dir     string  `json:"dir"`
	Parent  string  `json:"parent"`
	AtRoot  bool    `json:"atRoot"`
	Entries []Entry `json:"entries"`
}

// List reads a local directory. An empty dir uses the user home directory.
func List(dir string, showHidden bool) (Listing, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Listing{}, err
		}
		dir = home
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Listing{}, err
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return Listing{}, err
	}
	if !info.IsDir() {
		return Listing{}, fmt.Errorf("%s is not a directory", abs)
	}

	dirEntries, err := os.ReadDir(abs)
	if err != nil {
		return Listing{}, err
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, item := range dirEntries {
		name := item.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(abs, name)
		entry := Entry{Name: name, Path: full}
		st, statErr := os.Stat(full)
		if statErr != nil {
			if meta, infoErr := item.Info(); infoErr == nil {
				entry.Size = meta.Size()
				entry.ModTime = meta.ModTime().Format(time.RFC3339)
			}
			entries = append(entries, entry)
			continue
		}
		entry.IsDir = st.IsDir()
		entry.Size = st.Size()
		entry.ModTime = st.ModTime().Format(time.RFC3339)
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	parent := filepath.Dir(abs)
	return Listing{
		Dir:     abs,
		Parent:  parent,
		AtRoot:  parent == abs,
		Entries: entries,
	}, nil
}
