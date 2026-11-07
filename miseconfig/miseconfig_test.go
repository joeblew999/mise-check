package miseconfig

import (
	"strings"
	"testing"
)

// template is a tool's template .mise/config.toml, as a tool has it: its comment at the top, its settings, its pin.
const template = "# tool for this repo: its pin\n\n[settings]\n# Taken the minute they're out.\nminimum_release_age_excludes = [\"github:o/tool\"]\n# From GitHub.\nuse_versions_host = false\n\n[tools]\n\"github:o/tool\" = \"latest\"\n"

var tool = Tool{Name: "github:o/tool", Release: "1.2.3", Template: []byte(template), Init: "mise run tool:init"}

// TestKeep: a repo's own .mise/config.toml gets the tool's parts as this release writes them, and keeps the rest:
// another tool's entry and its own tools; the tool's older comment at the top, the older comment on the list and an
// older pin are written again. Then nothing is left to write, or to report. Another tool's file keeps its own comment
// at the top, and a pin of local stays.
func TestKeep(t *testing.T) {
	if p, err := tool.Pinned(); err != nil || !strings.Contains(string(p), `"github:o/tool" = "1.2.3"`) {
		t.Fatalf("Pinned: %v\n%s", err, p)
	}
	for _, c := range []struct{ own, want string }{
		{"# tool for this repo: older\n\n[settings]\n# The owner's tools: older\nminimum_release_age_excludes = [\"github:o/other\"]\n\n[tools]\n\"github:o/tool\" = \"1.2.0\"\nnode = \"24\"\n",
			"# tool for this repo: its pin\n\n[settings]\n# Taken the minute they're out.\nminimum_release_age_excludes = [\"github:o/other\", \"github:o/tool\"]\n# From GitHub.\nuse_versions_host = false\n\n[tools]\n\"github:o/tool\" = \"1.2.3\"\nnode = \"24\"\n"},
		{"# other for this repo\n\n[tools]\n\"github:o/tool\" = \"local\"\n",
			"# other for this repo\n\n[settings]\n# Taken the minute they're out.\nminimum_release_age_excludes = [\"github:o/tool\"]\n# From GitHub.\nuse_versions_host = false\n\n[tools]\n\"github:o/tool\" = \"local\"\n"},
	} {
		if p := tool.Problems([]byte(c.own)); len(p) == 0 {
			t.Errorf("Problems of %q: none", c.own)
		}
		out, wrote := tool.Keep([]byte(c.own))
		if string(out) != c.want {
			t.Errorf("Keep:\n%s\nwant:\n%s", out, c.want)
		}
		if len(wrote) == 0 {
			t.Errorf("Keep of %q said it wrote nothing", c.own)
		}
		if again, wrote := tool.Keep(out); string(again) != string(out) || len(wrote) > 0 {
			t.Errorf("Keep again wrote %q:\n%s", wrote, again)
		}
		if p := tool.Problems(out); len(p) > 0 {
			t.Errorf("Problems after Keep: %q", p)
		}
	}
}
