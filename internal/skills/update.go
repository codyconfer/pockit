package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/codyconfer/pockit/internal/cache"
)

func contentHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func addBundleState(r *Report) error {
	sort.Slice(r.Artifacts, func(i, j int) bool { return r.Artifacts[i].Path < r.Artifacts[j].Path })
	type digest struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	state := struct {
		Version     int      `json:"version"`
		Fingerprint string   `json:"fingerprint"`
		Artifacts   []digest `json:"artifacts"`
	}{Version: 1, Artifacts: []digest{}}
	for _, a := range r.Artifacts {
		state.Artifacts = append(state.Artifacts, digest{a.Path, contentHash(a.data)})
	}
	identity, err := json.Marshal(state.Artifacts)
	if err != nil {
		return err
	}
	state.Fingerprint = contentHash(identity)
	r.Fingerprint = state.Fingerprint
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	r.Artifacts = append(r.Artifacts, Artifact{Path: "bundle-state.json", Bytes: len(body), data: body})
	sort.Slice(r.Artifacts, func(i, j int) bool { return r.Artifacts[i].Path < r.Artifacts[j].Path })
	return nil
}

func checkBundle(r Report, out string) (Report, error) {
	root, err := os.OpenRoot(out)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return r, err
	}
	if root != nil {
		defer root.Close()
	}
	for _, a := range r.Artifacts {
		if !filepath.IsLocal(a.Path) {
			return r, ErrRefused
		}
		if err := cache.CheckPath(filepath.Join(out, filepath.FromSlash(a.Path))); err != nil {
			return r, fmt.Errorf("%w: symlink output", ErrRefused)
		}
		changed := root == nil
		if root != nil {
			info, err := root.Lstat(a.Path)
			switch {
			case errors.Is(err, os.ErrNotExist):
				changed = true
			case err != nil:
				return r, err
			case !info.Mode().IsRegular():
				return r, fmt.Errorf("%w: non-regular output", ErrRefused)
			default:
				body, err := root.ReadFile(a.Path)
				if err != nil {
					return r, err
				}
				changed = contentHash(body) != contentHash(a.data)
			}
		}
		if changed {
			r.Changed = append(r.Changed, a.Path)
		}
	}
	r.NeedsRebuild = len(r.Changed) > 0
	return r, nil
}
