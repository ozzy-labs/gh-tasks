package cmd

import (
	"strings"
	"testing"
)

func TestSanitizeDist_Rejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		".",
		"./",
		"..",
		"../",
		"/",
		"./.",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizeDist(in)
			if err == nil {
				t.Fatalf("sanitizeDist(%q) succeeded; want error", in)
			}
			if !strings.Contains(err.Error(), "refusing to use unsafe --dist value") {
				t.Errorf("error message missing guard prefix: %v", err)
			}
		})
	}
}

func TestSanitizeDist_Accepts(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"dist":         "dist",
		"./dist":       "dist",
		"build/dist":   "build/dist",
		"/tmp/foo":     "/tmp/foo",
		"./out/skills": "out/skills",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizeDist(in)
			if err != nil {
				t.Fatalf("sanitizeDist(%q) error: %v", in, err)
			}
			if got != want {
				t.Errorf("sanitizeDist(%q) = %q, want %q", in, got, want)
			}
		})
	}
}
