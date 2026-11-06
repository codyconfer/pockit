package cli

import (
	"fmt"
	"sort"

	"github.com/codyconfer/pockit/internal/fetching"
	"github.com/codyconfer/pockit/internal/planning"
	"github.com/codyconfer/pockit/internal/skills"
)

// reportSummary is presentation-only: the JSON report contracts stay unchanged.
func reportSummary(report any) []string {
	switch r := report.(type) {
	case fetching.Report:
		return []string{fmt.Sprintf("Sources: %d successful · %d failed · %d skipped", r.Success, r.Failed, r.Skipped)}
	case skills.Report:
		lines := []string{fmt.Sprintf("Artifacts: %d · Written: %d · Unavailable mappings: %d", len(r.Artifacts), r.Written, r.Failed)}
		if r.DryRun {
			lines = append(lines, "Dry run: no files written.")
		}
		return lines
	case planning.Report:
		lines := []string{fmt.Sprintf("Unique sources: %d", len(r.Entries))}
		keys := make([]string, 0, len(r.Summary))
		for key := range r.Summary {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s: %d", key, r.Summary[key]))
		}
		return lines
	case map[string]any:
		if count, ok := r["unique_sources"].(int); ok {
			return []string{fmt.Sprintf("Catalog valid · Unique sources: %d", count)}
		}
	}
	return nil
}
