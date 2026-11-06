package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/cache"
)

var ErrRefused = errors.New("config output safety refusal")

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("user config directory must be absolute")
	}
	return filepath.Join(base, "pockit"), nil
}

func InputPath(explicit, name string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return path, nil
}

func writeFiles(dir string, names []string, contents [][]byte, overwrite bool) ([]string, error) {
	if err := cache.CheckPath(dir); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRefused, err)
	}
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(dir, name)
		info, err := os.Lstat(paths[i])
		if err == nil && (!overwrite || !info.Mode().IsRegular()) {
			return nil, fmt.Errorf("%w: %s already exists or is not a regular file; use --overwrite for regular files", ErrRefused, paths[i])
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := cache.CheckPath(dir); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRefused, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for i, name := range names {
		if err := cache.Atomic(root, name, contents[i], overwrite); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func Install(overwrite bool) ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	return writeFiles(dir, []string{"sources.json", "roles.json"}, [][]byte{data.Catalog, data.Roles}, overwrite)
}

func ExportSchema(path string, overwrite bool) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	_, err = writeFiles(filepath.Dir(absolute), []string{filepath.Base(absolute)}, [][]byte{data.Schema}, overwrite)
	return err
}
