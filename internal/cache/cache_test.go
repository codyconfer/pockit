package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateAtomicCache(t *testing.T) {
	s, e := New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if s.Status(id) != "absent" {
		t.Fatal("new cache not absent")
	}
	if e = s.Write(id, Record{URL: "https://example.com", Outcome: "success"}, []byte("private")); e != nil {
		t.Fatal(e)
	}
	rec, b, e := s.Read(id)
	if e != nil || string(b) != "private" || rec.Digest == "" {
		t.Fatalf("read: %v %+v %q", e, rec, b)
	}
	info, e := os.Stat(filepath.Join(s.Path, rec.Digest+".content"))
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("broad cache permissions")
	}
	if e = os.WriteFile(filepath.Join(s.Path, rec.Digest+".content"), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Read(id); e == nil {
		t.Fatal("accepted corrupted content")
	}
	if _, _, e = s.Read("../../etc/passwd"); e == nil {
		t.Fatal("accepted traversal")
	}
}
func TestSymlinksAndNoClobber(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	link := filepath.Join(dir, "link")
	if e := os.Symlink(target, link); e != nil {
		t.Skip(e)
	}
	if _, e := New(filepath.Join(link, "cache")); e == nil {
		t.Fatal("symlink accepted")
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	if e = Atomic(root, "file", []byte("first"), false); e != nil {
		t.Fatal(e)
	}
	if e = Atomic(root, "file", []byte("second"), false); e == nil {
		t.Fatal("clobber allowed")
	}
	if e = Atomic(root, "../outside", nil, true); e == nil {
		t.Fatal("escape allowed")
	}
	if e = Atomic(root, "link/file", nil, true); e == nil {
		t.Fatal("symlink escape allowed")
	}
}
