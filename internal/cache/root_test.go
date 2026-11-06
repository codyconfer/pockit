package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingSharedDirectoryIsRefusedWithoutPermissionChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared")
	if e := os.Mkdir(path, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(path, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := New(path); e == nil {
		t.Fatal("shared cache directory accepted")
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0755 {
		t.Fatal("unrelated permissions changed")
	}
}
