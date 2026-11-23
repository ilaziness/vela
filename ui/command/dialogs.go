package commandui

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/layout"
)

// openAddDialog shows the custom-command form. Fields bind to st.addDraft
// so a cancelled dialog keeps nothing and a saved one resets.
func openAddDialog(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	st.addCat, st.addName, st.addDesc, st.addCmd = "", "", "", ""
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("command.add_title"),
		Width:      480,
		Height:     420,
		CustomView: func(win *gui.Window) gui.View {
			return addCommandView(win, d)
		},
	})
}

func addCommandView(w *gui.Window, d *Deps) gui.View {
	th := gui.CurrentTheme()
	st := gui.State[State](w)

	field := func(id, label, text string, onChange func(string)) gui.View {
		return gui.Input(gui.InputCfg{
			ID:          id,
			Label:       label,
			Text:        text,
			Placeholder: label,
			Sizing:      gui.FillFit,
			OnTextChanged: func(v string, ctx gui.EventCtx) {
				onChange(v)
				ctx.Consume()
			},
		})
	}

	cmdInput := gui.Input(gui.InputCfg{
		ID:         "cmd-add-cmd",
		Label:      i18n.T("command.command"),
		Text:       st.addCmd,
		Mode:       gui.InputMultiline,
		MinHeight:  72,
		Sizing:     gui.FillFit,
		Scrollable: true,
		OnTextChanged: func(v string, ctx gui.EventCtx) {
			gui.State[State](ctx.Window).addCmd = v
			ctx.Consume()
		},
	})

	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "cmd-add-form",
		Sizing:  gui.FillFit,
		Color:   th.ColorBackground,
		Padding: gui.PaddingMedium,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			field("cmd-add-cat", i18n.T("command.category"), st.addCat, func(v string) {
				gui.State[State](w).addCat = v
			}),
			field("cmd-add-name", i18n.T("command.name"), st.addName, func(v string) {
				gui.State[State](w).addName = v
			}),
			field("cmd-add-desc", i18n.T("command.description"), st.addDesc, func(v string) {
				gui.State[State](w).addDesc = v
			}),
			cmdInput,
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignEnd,
				Spacing: gui.SpacingSmall,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "cmd-add-cancel",
						Variant: gui.ButtonSecondary,
						Label:   i18n.T("common.cancel"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.DialogDismiss()
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "cmd-add-save",
						Variant: gui.ButtonPrimary,
						Label:   i18n.T("common.save"),
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							saveAddCommand(ctx.Window, d)
						},
					}),
				},
			})),
		},
	}))
}

// saveAddCommand validates the draft and persists it via the service.
// The dialog stays open on validation errors so the user can fix fields.
func saveAddCommand(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	cat := strings.TrimSpace(st.addCat)
	name := strings.TrimSpace(st.addName)
	cmdText := strings.TrimSpace(st.addCmd)
	if cat == "" || name == "" || cmdText == "" {
		toastErr(w, i18n.T("command.fields_required"))
		return
	}
	err := d.Commands.SaveUser(command.UserCommand{
		Category:    cat,
		Name:        name,
		Command:     cmdText,
		Description: strings.TrimSpace(st.addDesc),
	})
	if err != nil {
		toastErr(w, i18n.Tf("command.add_failed", errtext.Text(err)))
		return
	}
	st.addCat, st.addName, st.addDesc, st.addCmd = "", "", "", ""
	st.reload(d)
	w.DialogDismiss()
	toastOK(w, i18n.T("command.add_success"))
}
