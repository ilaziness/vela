package app

import (
	"testing"

	"github.com/ilaziness/vela/internal/bookmark"
)

func TestBookmarkTabLabel(t *testing.T) {
	cases := []struct {
		name string
		b    bookmark.Bookmark
		want string
	}{
		{name: "ssh bookmark", b: bookmark.Bookmark{Title: "生产库", User: "root"}, want: "生产库@root"},
		{name: "no user", b: bookmark.Bookmark{Title: "COM3"}, want: "COM3"},
		{name: "untitled falls back", b: bookmark.Bookmark{User: "root"}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bookmarkTabLabel(&tc.b); got != tc.want {
				t.Fatalf("bookmarkTabLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}
