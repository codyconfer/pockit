package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCorruptExistingContentCannotBecomeSuccess(t *testing.T) {
	store, e := New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte("url")))
	body := []byte("original")
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	rec := Record{URL: "https://example.com/a.md", Outcome: "success"}
	if e := store.Write(id, rec, body); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(store.Path, digest+".content"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := store.Write(id, rec, body); e == nil {
		t.Fatal("corruption reported as successful write")
	}
	if store.Status(id) != "invalid" {
		t.Fatal("corruption reported as successful read")
	}
}
