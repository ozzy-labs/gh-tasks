package cmd

import "testing"

// TestParseLinkArgs pins the contract of parseLinkArgs (cmd/link.go:176): it
// accepts exactly two positional issue numbers, both must parse via
// parseIssueNumber (which trims a leading '#', requires Atoi success, and
// rejects n<=0). The runLink caller maps a false return to error.link.argsRequired,
// so each rejection branch matters as a regression guard.
func TestParseLinkArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		wantPR   int
		wantTask int
		wantOK   bool
	}{
		{name: "happy", args: []string{"12", "42"}, wantPR: 12, wantTask: 42, wantOK: true},
		{name: "happy with hash", args: []string{"#12", "#42"}, wantPR: 12, wantTask: 42, wantOK: true},
		{name: "empty", args: nil, wantOK: false},
		{name: "single arg", args: []string{"12"}, wantOK: false},
		{name: "first non-numeric", args: []string{"abc", "42"}, wantOK: false},
		{name: "second non-numeric", args: []string{"12", "#xx"}, wantOK: false},
		{name: "first negative", args: []string{"-5", "42"}, wantOK: false},
		{name: "second zero", args: []string{"12", "0"}, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr, task, ok := parseLinkArgs(tc.args)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if pr != tc.wantPR || task != tc.wantTask {
				t.Errorf("got (%d, %d), want (%d, %d)", pr, task, tc.wantPR, tc.wantTask)
			}
		})
	}
}
