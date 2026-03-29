package handlers_test

import (
	"testing"

	"github.com/mouhamedsylla/kaal/internal/mcp/handlers"
)

func TestSplitTrim(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"api,db,cache", []string{"api", "db", "cache"}},
		{"api, db , cache", []string{"api", "db", "cache"}},
		{"api", []string{"api"}},
		{"", []string{}},
		{" , ", []string{}},
	}

	for _, tc := range cases {
		got := handlers.SplitTrim(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("SplitTrim(%q) = %v (len %d), want %v (len %d)",
				tc.input, got, len(got), tc.want, len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitTrim(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}
