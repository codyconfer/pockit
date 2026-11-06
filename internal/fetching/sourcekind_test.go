package fetching

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/codyconfer/pockit/internal/planning"
)

func TestMixedSourceResponsePolicies(t *testing.T) {
	for _, test := range []struct {
		name, mime, body, failure string
		actions                   []planning.Action
	}{
		{"script MIME union", "application/javascript", "// source", "", []planning.Action{planning.Document, planning.Script}},
		{"document refuses script MIME", "application/javascript", "// source", "unsupported response media type", []planning.Action{planning.Document}},
		{"reference vetoes HTML", "text/html", "<html>content</html>", "HTML returned instead of Markdown", []planning.Action{planning.Document, planning.Reference}},
		{"script vetoes HTML", "text/html", "<html>content</html>", "HTML returned instead of script", []planning.Action{planning.Document, planning.Script}},
		{"document accepts HTML", "text/html", "<html>content</html>", "", []planning.Action{planning.Document}},
		{"binary still refused", "application/octet-stream", "binary", "unsupported response media type", []planning.Action{planning.Document, planning.Script}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, raw := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.mime)
				io.WriteString(w, test.body)
			})
			e := entry(raw + "/source")
			e.Mappings = nil
			for _, action := range test.actions {
				e.Mappings = append(e.Mappings, planning.Mapping{Action: action})
			}
			report := f.Collect(context.Background(), planning.Report{Entries: []planning.Entry{e}})
			if test.failure == "" {
				if report.Success != 1 || report.Failed != 0 {
					t.Fatalf("unexpected failure: %+v", report)
				}
			} else if report.Failed != 1 || report.Outcomes[0].Detail != test.failure {
				t.Fatalf("want %q: %+v", test.failure, report)
			}
		})
	}
}
