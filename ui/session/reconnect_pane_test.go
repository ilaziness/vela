package session

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
)

func TestReconnectPaneStates(t *testing.T) {
	cases := []struct {
		name       string
		status     app.TabStatus
		wantConn   bool
		wantError  string
		wantButton bool
	}{
		{name: "disconnected", status: app.TabStatusDisconnected, wantButton: true},
		{name: "connecting", status: app.TabStatusConnecting, wantConn: true, wantButton: true},
		{
			name: "failed reconnect", status: app.TabStatusDisconnected,
			wantError: "host unreachable", wantButton: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &app.State{ActiveTab: "t1"}
			st.Tabs = []app.Tab{{
				ID: "t1", Kind: app.TabSession, Title: "box",
				Status: tc.status, ConnErr: tc.wantError,
			}}
			w := gui.NewTestWindow(t, gui.WindowCfg{Width: 800, Height: 600, State: st})
			root := w.TestRender(func(win *gui.Window) gui.View {
				return ReconnectPane(win, &app.Controller{}, st.TabByID("t1"))
			})

			if !hasText(root, i18n.T("shell.session.disconnected")) {
				t.Fatal("missing disconnected message")
			}
			if got := hasText(root, i18n.T("shell.session.connecting")); got != tc.wantConn {
				t.Fatalf("connecting message present = %v, want %v", got, tc.wantConn)
			}
			if tc.wantError != "" && !hasText(root, tc.wantError) {
				t.Fatal("missing connect error text")
			}
			if _, ok := findID(root, "tab-reconnect-t1"); ok != tc.wantButton {
				t.Fatalf("reconnect button present = %v, want %v", ok, tc.wantButton)
			}
		})
	}
}

func hasText(ly *gui.Layout, want string) bool {
	if ly == nil {
		return false
	}
	if ly.Shape != nil && ly.Shape.TC != nil && ly.Shape.TC.Text == want {
		return true
	}
	for i := range ly.Children {
		if hasText(&ly.Children[i], want) {
			return true
		}
	}
	return false
}
