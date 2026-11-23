package commandui

import (
	"sort"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/layout"
)

const libraryWidth float32 = 300

// node ID separators: leaf IDs embed both keys, so ':' (scope separator)
// and '/' stay out of the parts.
const (
	nodeCatPrefix = "cat:"
	nodeCmdSep    = "\x1f"
	nodeCmdPrefix = "cmd:"
)

// libraryPanel is the left column: command tree plus add/delete
// actions for user commands.
func libraryPanel(d *Deps, st *State) gui.View {
	th := gui.CurrentTheme()

	nodes := commandTreeNodes(st.all)

	deleteBtn := gui.Button(gui.ButtonCfg{
		ID:       "cmd-delete",
		Variant:  gui.ButtonGhost,
		Label:    i18n.T("common.delete"),
		Sizing:   gui.FillFit,
		Disabled: !st.selCustom,
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			confirmDelete(ctx.Window, d, st)
		},
	})

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "cmd-library",
		Sizing:  gui.FixedFill,
		Width:   libraryWidth,
		Color:   th.ColorPanel,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      i18n.T("command.library"),
				TextStyle: th.TextStyleTitle,
			}),
			gui.Tree(gui.TreeCfg{
				ID:     "cmd-tree",
				Sizing: gui.FillFill,
				Indent: 12,
				Nodes:  nodes,
				OnSelect: func(id string, ctx gui.EventCtx) {
					ctx.Consume()
					onTreeSelect(ctx.Window, d, st, id)
				},
			}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "cmd-add",
						Variant: gui.ButtonPrimary,
						Label:   i18n.T("command.add_custom"),
						Sizing:  gui.FillFit,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							openAddDialog(ctx.Window, d)
						},
					}),
					deleteBtn,
				},
			})),
		},
	}))
}

// commandTreeNodes flattens All() into category → command tree nodes.
// Categories and commands sort alphabetically for stable ordering.
func commandTreeNodes(all map[string][]command.Info) []gui.TreeNodeCfg {
	cats := make([]string, 0, len(all))
	for cat := range all {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	nodes := make([]gui.TreeNodeCfg, 0, len(cats))
	for _, cat := range cats {
		cmds := all[cat]
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
		kids := make([]gui.TreeNodeCfg, 0, len(cmds))
		for _, c := range cmds {
			text := c.Name
			if c.IsCustom {
				text += " *"
			}
			kids = append(kids, gui.TreeNodeCfg{
				ID:   nodeCmdPrefix + cat + nodeCmdSep + c.Name,
				Text: text,
				Icon: leafIcon(c.IsCustom),
			})
		}
		nodes = append(nodes, gui.TreeNodeCfg{
			ID:    nodeCatPrefix + cat,
			Text:  cat,
			Icon:  gui.IconFolder,
			Nodes: kids,
		})
	}
	return nodes
}

func leafIcon(custom bool) string {
	if custom {
		return gui.IconPencil
	}
	return gui.IconTerminal
}

// onTreeSelect fills the command input on leaf clicks and tracks the
// selection for the delete button. Category clicks only expand/collapse.
func onTreeSelect(w *gui.Window, d *Deps, st *State, id string) {
	if !strings.HasPrefix(id, nodeCmdPrefix) {
		return
	}
	rest := strings.TrimPrefix(id, nodeCmdPrefix)
	cat, name, ok := strings.Cut(rest, nodeCmdSep)
	if !ok {
		return
	}
	for _, info := range st.all[cat] {
		if info.Name == name {
			st.selCat, st.selName, st.selCustom = cat, name, info.IsCustom
			st.input = info.Command
			return
		}
	}
}

// confirmDelete asks before removing the selected user command.
func confirmDelete(w *gui.Window, d *Deps, st *State) {
	if !st.selCustom || st.selCat == "" {
		return
	}
	cat, name := st.selCat, st.selName
	w.Dialog(gui.DialogCfg{
		DialogType:    gui.DialogConfirm,
		Title:         i18n.T("command.confirm_delete_title"),
		Body:          i18n.Tf("command.confirm_delete_message", name),
		DefaultButton: gui.DialogButtonNo,
		OnOkYes: func(win *gui.Window) {
			if err := d.Commands.DeleteUser(cat, name); err != nil {
				toastErr(win, i18n.Tf("command.delete_failed", errtext.Text(err)))
				return
			}
			st.selCat, st.selName, st.selCustom = "", "", false
			st.reload(d)
			toastOK(win, i18n.T("command.delete_success"))
		},
	})
}
