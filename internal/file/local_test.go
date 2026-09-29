package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListLocalFilesFiltersHiddenAndSortsDirectoriesFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.txt"), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &Service{}
	visible, err := s.ListLocalFiles(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 2 {
		t.Fatalf("visible=%d want 2", len(visible))
	}
	if visible[0].Name != "folder" || visible[0].Type != dirType {
		t.Fatalf("first entry=%+v", visible[0])
	}

	all, err := s.ListLocalFiles(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("all=%d want 3", len(all))
	}
}
