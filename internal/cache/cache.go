package cache

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/codyconfer/pockit/internal/inheritance"
)

var validID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Record struct {
	Version  int                   `json:"version"`
	URL      string                `json:"url"`
	Outcome  string                `json:"outcome"`
	Detail   string                `json:"detail,omitempty"`
	Digest   string                `json:"digest,omitempty"`
	MIME     string                `json:"mime,omitempty"`
	Contexts []inheritance.Context `json:"contexts,omitempty"`
}
type Store struct{ Path string }

func DefaultPath() (string, error) {
	base, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(base, "pockit"), nil
}

func CheckPath(path string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	for p := abs; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path refused")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func New(path string) (*Store, error) {
	if path == "" {
		var e error
		path, e = DefaultPath()
		if e != nil {
			return nil, e
		}
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if e = CheckPath(abs); e != nil {
		return nil, e
	}
	for p := abs; ; p = filepath.Dir(p) {
		for _, mark := range []string{".git", "go.mod"} {
			if _, e := os.Stat(filepath.Join(p, mark)); e == nil {
				return nil, fmt.Errorf("cache must be outside source projects")
			}
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	if info, e := os.Lstat(abs); e == nil {
		if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("cache root must be a private directory (0700); existing permissions are never changed")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	return &Store{Path: abs}, nil
}
func (s *Store) root(create bool) (*os.Root, error) {
	if e := CheckPath(s.Path); e != nil {
		return nil, e
	}
	if create {
		if e := os.MkdirAll(s.Path, 0700); e != nil {
			return nil, e
		}
	}
	if e := CheckPath(s.Path); e != nil {
		return nil, e
	}
	info, e := os.Lstat(s.Path)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("cache root is not private")
	}
	return os.OpenRoot(s.Path)
}
func valid(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("invalid content identifier")
	}
	return nil
}
func (s *Store) Read(id string) (Record, []byte, error) {
	var rec Record
	if e := valid(id); e != nil {
		return rec, nil, e
	}
	root, e := s.root(false)
	if e != nil {
		return rec, nil, e
	}
	defer root.Close()
	b, e := root.ReadFile(id + ".json")
	if e != nil {
		return rec, nil, e
	}
	if e = json.Unmarshal(b, &rec); e != nil {
		return rec, nil, fmt.Errorf("invalid cache record")
	}
	if rec.Version != 1 {
		return rec, nil, fmt.Errorf("unsupported cache record")
	}
	if rec.Outcome != "success" {
		return rec, nil, nil
	}
	if e = valid(rec.Digest); e != nil {
		return rec, nil, fmt.Errorf("invalid cache digest")
	}
	body, e := root.ReadFile(rec.Digest + ".content")
	if e != nil {
		return rec, nil, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(body)) != rec.Digest {
		return rec, nil, fmt.Errorf("cache content identity mismatch")
	}
	return rec, body, nil
}
func (s *Store) Status(id string) string {
	r, _, e := s.Read(id)
	if errors.Is(e, os.ErrNotExist) {
		return "absent"
	}
	if e != nil {
		return "invalid"
	}
	return r.Outcome
}
func Atomic(root *os.Root, name string, b []byte, overwrite bool) error {
	if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return fmt.Errorf("unsafe output path")
	}
	nonce := make([]byte, 16)
	if _, e := rand.Read(nonce); e != nil {
		return e
	}
	tmp := ".pockit-" + hex.EncodeToString(nonce)
	f, e := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer root.Remove(tmp)
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if !overwrite {
		return root.Link(tmp, name)
	}
	if info, e := root.Lstat(name); e == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("nonregular output refused")
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	return root.Rename(tmp, name)
}
func (s *Store) Write(id string, rec Record, body []byte) error {
	if e := valid(id); e != nil {
		return e
	}
	root, e := s.root(true)
	if e != nil {
		return e
	}
	defer root.Close()
	rec.Version = 1
	if rec.Outcome == "success" {
		rec.Digest = fmt.Sprintf("%x", sha256.Sum256(body))
		if e := Atomic(root, rec.Digest+".content", body, false); e != nil {
			if !errors.Is(e, os.ErrExist) {
				return e
			}
			info, err := root.Lstat(rec.Digest + ".content")
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("existing cache content is not regular")
			}
			existing, err := root.ReadFile(rec.Digest + ".content")
			if err != nil {
				return err
			}
			if fmt.Sprintf("%x", sha256.Sum256(existing)) != rec.Digest {
				return fmt.Errorf("existing cache content identity mismatch")
			}
		}
	} else {
		rec.Digest = ""
	}
	b, e := json.Marshal(rec)
	if e != nil {
		return e
	}
	return Atomic(root, id+".json", b, true)
}
