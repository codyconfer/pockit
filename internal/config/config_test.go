package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/data"
)

func TestInstallAndDiscovery(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	path, err := InputPath("", "sources.json")
	if err != nil || path != "" {
		t.Fatal(path, err)
	}
	paths, err := Install(false)
	if err != nil || len(paths) != 2 {
		t.Fatal(paths, err)
	}
	for name, want := range map[string][]byte{"sources.json": data.Catalog, "roles.json": data.Roles} {
		path, err := InputPath("", name)
		if err != nil || path != filepath.Join(dir, name) {
			t.Fatal(path, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("wrong installed %s: %v", name, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("config file is not private: %v", err)
		}
	}
	if _, err := Install(false); !errors.Is(err, ErrRefused) {
		t.Fatal("overwrite allowed", err)
	}
	if _, err := Install(true); err != nil {
		t.Fatal(err)
	}
	if path, err := InputPath("explicit.json", "sources.json"); err != nil || path != "explicit.json" {
		t.Fatal(path, err)
	}
}

func TestInstallPreflight(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, _ := Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	roles := filepath.Join(dir, "roles.json")
	if err := os.WriteFile(roles, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(false); !errors.Is(err, ErrRefused) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sources.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preflight created sources", err)
	}
}

func TestSchemaExportSafety(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "schema.json")
	if err := ExportSchema(path, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data.Schema) {
		t.Fatal("schema mismatch", err)
	}
	if err := ExportSchema(path, false); !errors.Is(err, ErrRefused) {
		t.Fatal("overwrite allowed", err)
	}
	if err := ExportSchema(path, true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Skip(err)
	}
	if err := ExportSchema(link, true); !errors.Is(err, ErrRefused) {
		t.Fatal("symlink allowed", err)
	}
	parent := filepath.Join(dir, "linked-dir")
	if err := os.Symlink(filepath.Dir(path), parent); err != nil {
		t.Fatal(err)
	}
	if err := ExportSchema(filepath.Join(parent, "other.json"), false); !errors.Is(err, ErrRefused) {
		t.Fatal("symlink directory allowed", err)
	}
	if err := ExportSchema(filepath.Dir(path), true); !errors.Is(err, ErrRefused) {
		t.Fatal("directory replacement allowed", err)
	}
}
