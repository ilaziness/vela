package services

import "testing"

func TestSuggestedSaveName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/home/user/readme.md", "readme.md"},
		{"/home/user/project/", "project"},
		{"project", "project"},
		{"/", "download"},
		{"", "download"},
		{".", "download"},
	}
	for _, tc := range tests {
		if got := suggestedSaveName(tc.in); got != tc.want {
			t.Fatalf("suggestedSaveName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
