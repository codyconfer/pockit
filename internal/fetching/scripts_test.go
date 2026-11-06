package fetching

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/codyconfer/pockit/internal/planning"
)

func TestFetchScript(t *testing.T) {
	for _, mime := range []string{"text/plain", "application/vnd.github.raw+json", "application/x-sh"} {
		t.Run(mime, func(t *testing.T) {
			body := "#!/bin/sh\necho usable\n"
			f, raw := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", mime)
				io.WriteString(w, body)
			})
			e := entry(raw + "/tool.sh")
			e.Mappings[0].Action = planning.Script
			report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e}})
			if report.Failed != 0 || report.Success != 1 {
				t.Fatalf("%+v", report)
			}
			_, got, err := f.store.Read(e.ID)
			if err != nil || string(got) != body {
				t.Fatalf("script changed: %q %v", got, err)
			}
		})
	}
	f, raw := fixture(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>not a script</html>") })
	e := entry(raw + "/tool.sh")
	e.Mappings[0].Action = planning.Script
	report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e}})
	if report.Failed != 1 {
		t.Fatal("HTML script accepted")
	}
	target, err := RequestURL("https://github.com/example/repo/blob/main/scripts/tool.sh")
	if err != nil || target != "https://api.github.com/repos/example/repo/contents/scripts/tool.sh?ref=main" {
		t.Fatalf("%s %v", target, err)
	}
}
