package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/internal/config"
)

func TestUserConfigPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Install(false); err != nil {
		t.Fatal(err)
	}
	dir, _ := config.Dir()
	for name, body := range map[string]string{"sources.json": `{"custom":{"resources":[]}}`, "roles.json": `{"custom":[]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Load(Inputs{})
	if err != nil || len(s.Catalog) != 1 || s.Catalog["custom"].Resources == nil {
		t.Fatalf("user config not loaded: %v %v", s, err)
	}
	s, err = Load(Inputs{Embedded: true})
	if err != nil || len(s.Catalog) < 2 {
		t.Fatal("embedded inputs not isolated", err)
	}
	catalog := filepath.Join(t.TempDir(), "sources.json")
	roles := filepath.Join(t.TempDir(), "roles.json")
	if err := os.WriteFile(catalog, []byte(`{"override":{"resources":[]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roles, []byte(`{"override":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Load(Inputs{Catalog: catalog, Inheritance: roles})
	if err != nil || s.Catalog["override"].Resources == nil {
		t.Fatal("explicit inputs did not override", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources.json"), []byte(`invalid`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(Inputs{}); err == nil {
		t.Fatal("invalid installed config silently ignored")
	}
}

func TestMissingConfigFallsBack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(Inputs{}); err != nil {
		t.Fatal(err)
	}
}
