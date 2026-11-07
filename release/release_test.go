package release

import "testing"

// TestNext: the next release is the patch after the newest vMAJOR.MINOR.PATCH by number, other tags aside.
func TestNext(t *testing.T) {
	for want, tags := range map[string][]string{
		"v0.0.1":  nil,
		"v0.4.10": {"v0.4.9", "v0.3.20", "latest", "v1.0", "v0.4.9-rc1"},
		"v1.0.1":  {"v0.9.9", "v1.0.0"},
	} {
		if got := Next(tags); got != want {
			t.Errorf("Next(%q) = %s, want %s", tags, got, want)
		}
	}
}
