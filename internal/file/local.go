package file

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type HostFileSystemInfo struct {
	Home      string   `json:"home"`
	Roots     []string `json:"roots"`
	Separator string   `json:"separator"`
	OS        string   `json:"os"`
}

func (s *Service) GetHostFileSystemInfo() (HostFileSystemInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return HostFileSystemInfo{}, err
	}
	return HostFileSystemInfo{
		Home:      home,
		Roots:     hostRoots(),
		Separator: string(filepath.Separator),
		OS:        runtime.GOOS,
	}, nil
}

func (s *Service) ListLocalFiles(localPath string, showHidden bool) ([]Entry, error) {
	target := strings.TrimSpace(localPath)
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		target = home
	}
	target = filepath.Clean(target)

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}

	result := make([]Entry, 0, len(entries))
	for _, dirEntry := range entries {
		name := dirEntry.Name()
		hidden := strings.HasPrefix(name, ".")
		if !showHidden && hidden {
			continue
		}

		fullPath := filepath.Join(target, name)
		info, err := dirEntry.Info()
		if err != nil {
			continue
		}

		entryType := regularType
		size := info.Size()
		sizeHuman := formatFileSize(size)
		if dirEntry.Type()&os.ModeSymlink != 0 {
			entryType = symlinkType
		} else if info.IsDir() {
			entryType = dirType
			size = 0
			sizeHuman = sizeDirNone
		} else if !info.Mode().IsRegular() {
			entryType = otherType
		}

		result = append(result, Entry{
			Name:        name,
			Path:        fullPath,
			Type:        entryType,
			Size:        size,
			SizeHuman:   sizeHuman,
			Permissions: info.Mode().String(),
			ModifiedAt:  info.ModTime().Format("2006-01-02 15:04"),
			IsHidden:    hidden,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Type == dirType && result[j].Type != dirType {
			return true
		}
		if result[i].Type != dirType && result[j].Type == dirType {
			return false
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func hostRoots() []string {
	if runtime.GOOS != "windows" {
		return []string{string(filepath.Separator)}
	}

	roots := make([]string, 0, 4)
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := string(letter) + ":\\"
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}


func (s *Service) GetLocalParentPath(localPath string) string {
	target := filepath.Clean(strings.TrimSpace(localPath))
	if target == "." || target == "" {
		if home, err := os.UserHomeDir(); err == nil {
			target = filepath.Clean(home)
		}
	}
	parent := filepath.Dir(target)
	if parent == "." {
		return target
	}
	return parent
}
