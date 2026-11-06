package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/skills"
)

// Installation follows the same no-symlink policy as generated output.
func installPath(path string) error {
	if err := cache.CheckPath(path); err != nil {
		return fmt.Errorf("%w: %v", skills.ErrRefused, err)
	}
	return nil
}

func installOpenCode(snippet json.RawMessage) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("user config directory must be absolute")
	}
	dir := filepath.Join(base, "opencode")
	path := filepath.Join(dir, "opencode.json")
	if err := installPath(path); err != nil {
		return "", err
	}
	// Do not silently shadow a JSONC config or strip its comments.
	if _, err := os.Lstat(filepath.Join(dir, "opencode.jsonc")); err == nil {
		return "", fmt.Errorf("opencode.jsonc exists; merge the printed installation.config manually")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	config := map[string]json.RawMessage{}
	exists := false
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%w: OpenCode config is not a regular file", skills.ErrRefused)
		}
		exists = true
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(body, &config); err != nil || config == nil {
			return "", fmt.Errorf("OpenCode config must be a JSON object; merge the printed installation.config manually")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var generated struct {
		Plugin []string `json:"plugin"`
	}
	if err := json.Unmarshal(snippet, &generated); err != nil || len(generated.Plugin) != 1 {
		return "", fmt.Errorf("invalid generated OpenCode installation config")
	}
	key := "plugin"
	if _, ok := config["plugins"]; ok {
		key = "plugins"
	}
	var plugins []json.RawMessage
	if raw, ok := config[key]; ok {
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &plugins) != nil {
			return "", fmt.Errorf("OpenCode %s must be an array; merge the printed installation.config manually", key)
		}
	}
	// Check both accepted keys to avoid registering the same URL twice.
	for _, k := range []string{"plugin", "plugins"} {
		var entries []json.RawMessage
		if raw, ok := config[k]; ok {
			if json.Unmarshal(raw, &entries) != nil {
				return "", fmt.Errorf("OpenCode %s must be an array", k)
			}
			for _, entry := range entries {
				var value string
				if json.Unmarshal(entry, &value) == nil && value == generated.Plugin[0] {
					return path, nil
				}
			}
		}
	}
	url, _ := json.Marshal(generated.Plugin[0])
	plugins = append(plugins, url)
	config[key], _ = json.Marshal(plugins)
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := installPath(path); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := cache.Atomic(root, "opencode.json", append(body, '\n'), exists); err != nil {
		return "", err
	}
	return path, nil
}

func installGoose(out, name string, overwrite bool) (string, error) {
	if name == "" {
		name = skills.DefaultPackageName
	}
	if err := skills.ValidatePackageName(name); err != nil || strings.ContainsAny(name, "@/\\") {
		return "", fmt.Errorf("invalid Goose plugin name")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return installLocalBundle(out, filepath.Join(home, ".agents", "plugins", name), "Goose", overwrite)
}

func installCursor(out, name string, overwrite bool) (string, error) {
	if name == "" {
		name = skills.DefaultPackageName
	}
	if err := skills.ValidateCursorPackageName(name); err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return installLocalBundle(out, filepath.Join(home, ".cursor", "plugins", "local", name), "Cursor", overwrite)
}

// Stage plugin copies without modifying host settings or invoking host CLIs.
func stageSkillPlugin(out, name string, overwrite bool, codex bool) (string, error) {
	if name == "" {
		name = skills.DefaultPackageName
	}
	kind := skills.ClaudeCodePackage
	host, directory := "Claude Code", ".claude"
	if codex {
		kind, host, directory = skills.CodexPackage, "Codex", ".codex"
	}
	opts := skills.Options{ClaudeCodePlugin: kind == skills.ClaudeCodePackage, CodexPlugin: kind == skills.CodexPackage}
	adapter, err := skills.PackageAdapterFor(opts)
	if err != nil {
		return "", err
	}
	if err := adapter.ValidateName(name); err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return installLocalBundle(out, filepath.Join(home, directory, "plugins", "local", name), host, overwrite)
}

// Local bundles install complete copies with identical safety checks.
func installLocalBundle(out, dest, host string, overwrite bool) (string, error) {
	source, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	if source == dest || strings.HasPrefix(dest, source+string(filepath.Separator)) || strings.HasPrefix(source, dest+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s output and installation directories must not overlap", skills.ErrRefused, host)
	}
	if err := installPath(source); err != nil {
		return "", err
	}
	if err := installPath(dest); err != nil {
		return "", err
	}
	if info, err := os.Lstat(dest); err == nil {
		if !overwrite || !info.IsDir() {
			return "", fmt.Errorf("%w: %s installation exists; use --overwrite for an existing directory", skills.ErrRefused, host)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Preflight every source and destination before changing any installed files.
	var files []string
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("%w: nonregular %s bundle file", skills.ErrRefused, host)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if err := installPath(target); err != nil {
			return err
		}
		if info, err := os.Lstat(target); err == nil {
			if entry.IsDir() != info.IsDir() || (!entry.IsDir() && !info.Mode().IsRegular()) {
				return fmt.Errorf("%w: incompatible %s installation path", skills.ErrRefused, host)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if !entry.IsDir() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0700); err != nil {
		return "", err
	}
	if err := installPath(dest); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return "", err
	}
	defer root.Close()
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(source, rel))
		if err != nil {
			return "", err
		}
		if err := root.MkdirAll(filepath.Dir(rel), 0700); err != nil {
			return "", err
		}
		if err := installPath(filepath.Join(dest, rel)); err != nil {
			return "", err
		}
		if err := cache.Atomic(root, rel, body, overwrite); err != nil {
			return "", err
		}
	}
	return dest, nil
}
