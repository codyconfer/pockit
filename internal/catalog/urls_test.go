package catalog

import (
	"strings"
	"testing"
)

func TestSecretURLsNeverEnterDiagnostics(t *testing.T) {
	for _, raw := range []string{"https://user:supersecret@example.com/a", "https://example.com/?token=supersecret", "https://example.com/?api_key=supersecret", "https://example.com/?X-Amz-Signature=supersecret", "https://example.com/?sig=supersecret"} {
		if e := CheckURLSecrets(raw); e == nil || strings.Contains(e.Error(), "supersecret") {
			t.Fatalf("unsafe error: %v", e)
		}
		_, e := Parse([]byte(`{"role":{"resources":[{"url":"`+raw+`","kind":"html"}]}}`), []byte(`{"type":"object"}`))
		if e == nil || strings.Contains(e.Error(), "supersecret") {
			t.Fatalf("unsafe validation diagnostic: %v", e)
		}
	}
}
