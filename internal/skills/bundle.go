package skills

import (
	"html"
	"net/url"
	"sort"
	"strings"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
	"github.com/codyconfer/pockit/internal/sourcekind"
)

type BundleSource struct {
	ID                  string   `json:"id"`
	URL                 string   `json:"url"`
	RelativeLinkBaseURL string   `json:"relative_link_base_url,omitempty"`
	MIME                string   `json:"mime,omitempty"`
	LocalPaths          []string `json:"local_paths"`
	CacheStatus         string   `json:"cache_status"`
	ContentSHA256       string   `json:"content_sha256,omitempty"`
	InlineAnchor        string   `json:"inline_anchor,omitempty"`
}

func documentPath(id, contentType string) string { return sourcekind.DocumentPath(id, contentType) }

func bundleSource(entry planning.Entry, rec cache.Record, files map[string][]byte) BundleSource {
	s := BundleSource{ID: entry.ID, URL: entry.URL, CacheStatus: entry.CacheStatus, LocalPaths: []string{}}
	if u, err := url.Parse(entry.URL); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		u.Fragment = ""
		u.RawFragment = ""
		s.RelativeLinkBaseURL = u.String()
	}
	if rec.URL == entry.URL && rec.Outcome == "success" {
		s.MIME = rec.MIME
	}
	seen := map[string]bool{}
	for _, mapping := range entry.Mappings {
		name := sourcekind.ForAction(mapping.Action).LocalPath(entry.ID, entry.URL, rec.MIME)
		if name != "" && !seen[name] && hasArtifact(files, name) {
			s.LocalPaths = append(s.LocalPaths, name)
			seen[name] = true
		}
	}
	if len(s.LocalPaths) > 0 {
		s.ContentSHA256 = rec.Digest
	}
	sort.Strings(s.LocalPaths)
	return s
}

func sourceLabel(entry planning.Entry) string {
	contexts := []string{}
	for _, mapping := range entry.Mappings {
		if label := strings.Join(strings.Fields(mapping.Context.Source.Context), " "); label != "" {
			contexts = append(contexts, label)
		}
	}
	sort.Strings(contexts)
	label := entry.URL
	if len(contexts) > 0 {
		label = contexts[0]
	}
	if label == "" {
		label = entry.ID
	}
	runes := []rune(label)
	if len(runes) > 120 {
		label = string(runes[:117]) + "..."
	}
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "`", "\\`", "*", "\\*", "_", "\\_").Replace(html.EscapeString(label))
}

func hasArtifact(artifacts map[string][]byte, name string) bool {
	_, ok := artifacts[name]
	return ok
}

const scriptPrerequisites = `# Script prerequisites

Packaged scripts preserve original bytes. They are source snapshots, not standalone
executables, and PocKit neither runs them nor packages their repository dependencies.

Before running a script:

1. Find its original URL and local path in ../bundle-index.json; inspect the script
   and consult the source repository's documentation. Follow source links, not the
   hashed bundle location, to identify its original repository-relative path.
2. Obtain an authorized checkout of that repository at the intended revision and
   verify the script against it. Run the repository's original script at its original
   path, not the bundled snapshot. Scripts may derive their root from their own path;
   changing the working directory alone does not supply missing dependencies.
3. Verify every invoked helper, configuration file, package manifest, runtime and
   tool version in that checkout. Helpers, infra directories and package.json files
   are not automatically bundled. Resolve missing prerequisites before execution.
4. Confirm credentials, account/profile, target environment, and explicit operational
   authorization. Inspect authentication resets, cloud/Vault/Pulumi access, secret
   retrieval, generated files, deletions and package-manager install hooks as applicable.
   Reference inheritance grants no credentials or authority.

Do not execute a bundled snapshot directly or invent missing helpers. If the source
checkout or authorization is unavailable, treat the script as read-only reference data.
`
