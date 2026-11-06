package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestPackageName(t *testing.T) {
	for _, name := range []string{"pockit-skills", "my.skills_v2", "@team/my-skills"} {
		if err := ValidatePackageName(name); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
	for _, name := range []string{"", "BadName", "../bad", "@team", "@team/", "bad name", "bad\"name", ".hidden", "node_modules", "favicon.ico", strings.Repeat("a", 215)} {
		if err := ValidatePackageName(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"", "custom-skills", "@team/custom-skills"} {
		r, err := Build(context.Background(), planning.Report{}, &cache.Store{}, []string{"writer"}, Options{PiExtension: true, PackageName: name})
		if err != nil {
			t.Fatal(err)
		}
		want := name
		if want == "" {
			want = DefaultPackageName
		}
		found := false
		for _, artifact := range r.Artifacts {
			if artifact.Path != "package.json" {
				continue
			}
			var manifest struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(artifact.data, &manifest); err != nil || manifest.Name != want {
				t.Fatalf("manifest=%s err=%v", artifact.data, err)
			}
			found = true
		}
		if !found {
			t.Fatal("missing package manifest")
		}
	}
}

func TestPackageSemVer(t *testing.T) {
	for _, version := range []string{"0.0.0", "1.2.3", "10.20.30-rc.1", "1.0.0-alpha-beta.0+build.001"} {
		if err := ValidatePackageVersion(version); err != nil {
			t.Errorf("%q: %v", version, err)
		}
	}
	for _, version := range []string{"", "v1.0.0", "1.2", "01.2.3", "1.2.3-01", "1.2.3-rc.01", "1.2.3-", "1.2.3+", "1.2.3\n", "1.2.3\"", "1.2.3-α"} {
		if err := ValidatePackageVersion(version); err == nil {
			t.Errorf("accepted %q", version)
		}
	}
}

func TestPiPackageVersionedSnapshot(t *testing.T) {
	ctx := context.Background()
	opts := Options{Out: filepath.Join(t.TempDir(), "bundle"), PiExtension: true, PackageVersion: "2.1.0-rc.1+build.2"}
	store := &cache.Store{}
	first, err := Generate(ctx, planning.Report{}, store, []string{"writer"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range first.Artifacts {
		if artifact.Path == "package.json" && !bytes.Contains(artifact.data, []byte(`"version": "2.1.0-rc.1+build.2"`)) {
			t.Fatal("missing manifest version")
		}
		if artifact.Path == "CHANGELOG.md" && !bytes.Contains(artifact.data, []byte("## [2.1.0-rc.1+build.2]")) {
			t.Fatal("missing Pi changelog heading")
		}
	}
	opts.Check = true
	same, err := Generate(ctx, planning.Report{}, store, []string{"writer"}, opts)
	if err != nil || same.NeedsRebuild || same.Fingerprint != first.Fingerprint {
		t.Fatalf("snapshot not deterministic: %+v %v", same, err)
	}
	opts.PackageVersion = "2.1.0"
	changed, err := Generate(ctx, planning.Report{}, store, []string{"writer"}, opts)
	if err != nil || !changed.NeedsRebuild || changed.Fingerprint == first.Fingerprint {
		t.Fatalf("version change not detected: %+v %v", changed, err)
	}
	for _, opts := range []Options{{PackageVersion: "1.0.0"}, {PiExtension: true, PackageVersion: "invalid"}, {PackageName: "custom-skills"}, {PiExtension: true, PackageName: "../invalid"}} {
		if _, err := Build(ctx, planning.Report{}, store, []string{"writer"}, opts); err == nil {
			t.Fatalf("accepted invalid options: %+v", opts)
		}
	}
}
