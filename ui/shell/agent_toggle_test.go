package shell_test

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/shell"
)

// TestAgentToggleKeepsStageIDs guards the unconditional splitter: it puts the
// stage under the "agent-split"/"pane:first" scopes, and focus, blur tracking
// and scroll offsets are keyed by effective ID, so building it only while the
// assistant is open re-keyed the whole stage on every toggle.
func TestAgentToggleKeepsStageIDs(t *testing.T) {
	st := &app.State{}
	w := gui.NewTestWindow(t, gui.WindowCfg{
		Width:  1000,
		Height: 700,
		State:  st,
	})
	stageIDs := func() map[string]struct{} {
		w.TestRender(func(win *gui.Window) gui.View {
			return shell.Root(win, &app.Controller{}, shell.Chrome{})
		})
		ids := make(map[string]struct{})
		for _, id := range w.EffectiveIDs() {
			ids[id] = struct{}{}
		}
		return ids
	}

	closed := stageIDs()
	// The stage is scoped even with the assistant closed; an unscoped
	// "session-tabs" here means the splitter came back conditionally.
	if _, ok := closed["agent-split:pane:first:session-tabs"]; !ok {
		t.Fatalf("stage is not under the splitter scope while the assistant is closed")
	}

	st.AgentOpen = true
	open := stageIDs()
	st.AgentOpen = false
	closedAgain := stageIDs()

	for _, step := range []struct {
		name   string
		before map[string]struct{}
		after  map[string]struct{}
	}{
		{"opening the assistant", closed, open},
		{"closing it again", closed, closedAgain},
	} {
		for id := range step.before {
			if _, ok := step.after[id]; !ok {
				t.Errorf("%s dropped stage ID %q", step.name, id)
			}
		}
	}
}
