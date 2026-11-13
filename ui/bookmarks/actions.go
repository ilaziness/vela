package bookmarks

import (
	"os"
	"strings"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

func reload(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	groups, err := d.Bookmarks.ListGrouped()
	if err != nil {
		toastErrText(w, i18n.Tf("bookmark.load_failed", errText(err)))
		return
	}
	st.Groups = groups
	if d.Keys != nil {
		if keys, err := d.Keys.List(); err == nil {
			st.Keys = keys
		}
	}
}

func loadDraftFromBookmark(st *State, b bookmark.Bookmark) {
	st.Draft = b
	// Clone tags so editor mutations do not alias the tree's slice.
	if b.Tags != nil {
		st.Draft.Tags = append([]string(nil), b.Tags...)
	}
	st.EnvText = formatEnvLines(b.EnvVars)
	st.HasPassword = b.Password == bookmark.PasswordMask
	st.HasKeyPassword = b.PrivateKeyPassword == bookmark.PasswordMask
	st.HasProxyPass = b.ProxyPassword == bookmark.PasswordMask
	st.Editing = true
	st.Dirty = false
	st.ActiveTab = tabBasic
	st.SelKind = selBookmark
	st.SelID = b.ID
}

func findBookmark(st *State, id string) *bookmark.Bookmark {
	for _, g := range st.Groups {
		if g == nil {
			continue
		}
		for i := range g.Bookmarks {
			if g.Bookmarks[i].ID == id {
				b := g.Bookmarks[i]
				return &b
			}
		}
	}
	return nil
}

func currentGroupName(st *State) string {
	switch st.SelKind {
	case selGroup:
		if st.SelID != "" {
			return st.SelID
		}
	case selBookmark:
		if st.Editing && st.Draft.GroupName != "" {
			return st.Draft.GroupName
		}
		if b := findBookmark(st, st.SelID); b != nil {
			return b.GroupName
		}
	}
	return bookmark.DefaultGroupName
}

func groupBookmarkCount(st *State, name string) int {
	for _, g := range st.Groups {
		if g != nil && g.Name == name {
			return len(g.Bookmarks)
		}
	}
	return 0
}

func prepareDraftForSave(st *State) (bookmark.Bookmark, error) {
	b := st.Draft
	if b.Tags != nil {
		b.Tags = append([]string(nil), b.Tags...)
	}
	title := strings.TrimSpace(b.Title)
	if title == "" {
		return b, usererr.Coded("bookmark.name_required", "Bookmark name is required")
	}
	b.Title = title
	if b.GroupName == "" {
		b.GroupName = bookmark.DefaultGroupName
	}
	if b.Protocol == "ssh" {
		if err := bookmark.ValidateProxy(b); err != nil {
			return b, err
		}
		m, err := parseEnvLines(st.EnvText)
		if err != nil {
			return b, err
		}
		raw, err := bookmark.FormatEnvVars(m)
		if err != nil {
			return b, err
		}
		b.EnvVars = raw
	} else {
		b.EnvVars = ""
		b.StartupCmd = ""
		b.Term = ""
	}
	return b, nil
}

func doSave(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	if st.Busy || !st.Editing {
		return
	}
	st.Busy = true
	withVault(w, d, i18n.T("password.encrypt_bookmark"), func() {
		st := gui.State[State](w)
		b, err := prepareDraftForSave(st)
		if err != nil {
			st.Busy = false
			toastErr(w, err)
			return
		}
		go func() {
			id, err := d.Bookmarks.Save(b)
			w.QueueCommand(func(win *gui.Window) {
				s := gui.State[State](win)
				s.Busy = false
				if err != nil {
					toastErr(win, err)
					return
				}
				reload(win, d)
				if nb := findBookmark(s, id); nb != nil {
					loadDraftFromBookmark(s, *nb)
				} else {
					s.Draft.ID = id
					s.Dirty = false
					s.SelKind = selBookmark
					s.SelID = id
				}
				toastOK(win, i18n.T("bookmark.save_success"))
			})
		}()
	}, func(win *gui.Window) {
		gui.State[State](win).Busy = false
	})
}

func doTest(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	if st.Busy || !st.Editing {
		return
	}
	st.Busy = true
	withVault(w, d, i18n.T("password.encrypt_decrypt_required"), func() {
		st := gui.State[State](w)
		b, err := prepareDraftForSave(st)
		if err != nil {
			st.Busy = false
			toastErr(w, err)
			return
		}
		d.Ctrl.TestBookmark(b, func(err error) {
			w.QueueCommand(func(win *gui.Window) {
				s := gui.State[State](win)
				s.Busy = false
				if err != nil {
					toastErrText(win, i18n.Tf("bookmark.test_failed", errText(err)))
					return
				}
				toastOK(win, i18n.T("bookmark.test_success"))
			})
		})
	}, func(win *gui.Window) {
		gui.State[State](win).Busy = false
	})
}

func confirmIfDirty(w *gui.Window, then func()) {
	st := gui.State[State](w)
	if !st.Dirty {
		then()
		return
	}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("bookmark.unsaved_title"),
		Body:       i18n.T("bookmark.unsaved_body"),
		OnOkYes: func(win *gui.Window) {
			then()
		},
	})
}

func selectTreeNode(w *gui.Window, d *Deps, nodeID string) {
	kind, key := parseNode(nodeID)
	confirmIfDirty(w, func() {
		st := gui.State[State](w)
		switch kind {
		case selGroup:
			st.SelKind = selGroup
			st.SelID = key
			st.Editing = false
			st.Dirty = false
			st.Draft = bookmark.Bookmark{}
			st.EnvText = ""
		case selBookmark:
			b := findBookmark(st, key)
			if b == nil {
				mb, err := d.Bookmarks.GetMasked(key)
				if err != nil {
					toastErr(w, err)
					return
				}
				if mb == nil {
					return
				}
				loadDraftFromBookmark(st, *mb)
				return
			}
			loadDraftFromBookmark(st, *b)
		}
	})
}

func addBookmarkDraft(w *gui.Window) {
	confirmIfDirty(w, func() {
		st := gui.State[State](w)
		group := currentGroupName(st)
		n := groupBookmarkCount(st, group) + 1
		st.Draft = emptySSHBookmark(group, n)
		st.EnvText = ""
		st.TagDraft = ""
		st.HasPassword = false
		st.HasKeyPassword = false
		st.HasProxyPass = false
		st.Editing = true
		st.Dirty = true
		st.ActiveTab = tabBasic
		st.SelKind = selGroup
		st.SelID = group
	})
}

func promptAddGroup(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	st.DialogName = ""
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.add_group"),
		Width:      360,
		CustomView: func(win *gui.Window) gui.View {
			s := gui.State[State](win)
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: []gui.View{
					gui.Input(gui.InputCfg{
						ID:          "bm-add-group-name",
						Label:       i18n.T("bookmark.group_name_placeholder"),
						Text:        s.DialogName,
						Placeholder: i18n.T("bookmark.group_name_placeholder"),
						Sizing:      gui.FillFit,
						OnTextChanged: func(v string, ctx gui.EventCtx) {
							gui.State[State](ctx.Window).DialogName = v
							ctx.Consume()
						},
					}),
					actionsRight(
						gui.Button(gui.ButtonCfg{
							ID:      "bm-add-group-cancel",
							Variant: gui.ButtonSecondary,
							Label:   i18n.T("common.cancel"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
							},
						}),
						gui.Button(gui.ButtonCfg{
							ID:      "bm-add-group-ok",
							Variant: gui.ButtonPrimary,
							Label:   i18n.T("common.ok"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								name := strings.TrimSpace(gui.State[State](ctx.Window).DialogName)
								if name == "" {
									return
								}
								ctx.Window.DialogDismiss()
								doAddGroup(ctx.Window, d, name)
							},
						}),
					),
				},
			})
		},
	})
}

func doAddGroup(w *gui.Window, d *Deps, name string) {
	if err := d.Bookmarks.AddGroup(name); err != nil {
		toastErrText(w, i18n.Tf("bookmark.add_group_failed", errText(err)))
		return
	}
	reload(w, d)
	st := gui.State[State](w)
	st.SelKind = selGroup
	st.SelID = name
	st.Editing = false
}

func promptRenameGroup(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	if st.SelKind != selGroup || isDefaultGroup(st.SelID) {
		return
	}
	old := st.SelID
	st.DialogName = old
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.rename_group"),
		Width:      360,
		CustomView: func(win *gui.Window) gui.View {
			s := gui.State[State](win)
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: []gui.View{
					gui.Input(gui.InputCfg{
						ID:     "bm-rename-group-name",
						Label:  i18n.T("bookmark.group_name_placeholder"),
						Text:   s.DialogName,
						Sizing: gui.FillFit,
						OnTextChanged: func(v string, ctx gui.EventCtx) {
							gui.State[State](ctx.Window).DialogName = v
							ctx.Consume()
						},
					}),
					actionsRight(
						gui.Button(gui.ButtonCfg{
							ID:      "bm-rename-group-cancel",
							Variant: gui.ButtonSecondary,
							Label:   i18n.T("common.cancel"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
							},
						}),
						gui.Button(gui.ButtonCfg{
							ID:      "bm-rename-group-ok",
							Variant: gui.ButtonPrimary,
							Label:   i18n.T("common.ok"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								name := strings.TrimSpace(gui.State[State](ctx.Window).DialogName)
								ctx.Window.DialogDismiss()
								if name == "" || name == old {
									return
								}
								doRenameGroup(ctx.Window, d, old, name)
							},
						}),
					),
				},
			})
		},
	})
}

func doRenameGroup(w *gui.Window, d *Deps, oldName, newName string) {
	icon := ""
	st := gui.State[State](w)
	for _, g := range st.Groups {
		if g != nil && g.Name == oldName {
			icon = g.Icon
			break
		}
	}
	if err := d.Bookmarks.UpdateGroup(oldName, newName, icon); err != nil {
		toastErrText(w, i18n.Tf("bookmark.rename_group_failed", errText(err)))
		return
	}
	reload(w, d)
	st = gui.State[State](w)
	st.SelKind = selGroup
	st.SelID = newName
	if st.Editing && st.Draft.GroupName == oldName {
		st.Draft.GroupName = newName
	}
}

func confirmDeleteGroup(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	if st.SelKind != selGroup || isDefaultGroup(st.SelID) {
		return
	}
	name := st.SelID
	confirmIfDirty(w, func() {
		w.Dialog(gui.DialogCfg{
			DialogType: gui.DialogConfirm,
			Title:      i18n.T("bookmark.delete_group"),
			Body:       displayGroupName(name),
			OnOkYes: func(win *gui.Window) {
				if err := d.Bookmarks.DeleteGroup(name); err != nil {
					toastErrText(win, i18n.Tf("bookmark.delete_group_failed", errText(err)))
					return
				}
				reload(win, d)
				gui.State[State](win).clearSelection()
			},
		})
	})
}

func confirmDeleteBookmark(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	id := ""
	title := ""
	if st.Editing && st.Draft.ID != "" {
		id = st.Draft.ID
		title = st.Draft.Title
	} else if st.SelKind == selBookmark {
		id = st.SelID
		if b := findBookmark(st, id); b != nil {
			title = b.Title
		}
	}
	if id == "" {
		// Unsaved draft: just discard.
		if st.Editing && st.Draft.ID == "" {
			st.clearSelection()
		}
		return
	}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("bookmark.delete_bookmark"),
		Body:       title,
		OnOkYes: func(win *gui.Window) {
			if err := d.Bookmarks.Delete(id); err != nil {
				toastErrText(win, i18n.Tf("bookmark.delete_failed", errText(err)))
				return
			}
			reload(win, d)
			gui.State[State](win).clearSelection()
		},
	})
}

func doCopyBookmark(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	id := ""
	if st.Editing && st.Draft.ID != "" {
		id = st.Draft.ID
	} else if st.SelKind == selBookmark {
		id = st.SelID
	}
	if id == "" {
		return
	}
	confirmIfDirty(w, func() {
		copied, err := d.Bookmarks.Copy(id)
		if err != nil {
			toastErrText(w, i18n.Tf("bookmark.copy_failed", errText(err)))
			return
		}
		reload(w, d)
		if copied != nil {
			loadDraftFromBookmark(gui.State[State](w), *copied)
			toastOK(w, i18n.T("bookmark.copy_success"))
		}
	})
}

func abortImport(st *State) {
	st.ImportGen++
	st.ImportBusy = false
	st.Busy = false
}

func promptImport(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	st.ImportGroup = bookmark.ImportedGroupName
	st.ImportBusy = false
	st.ImportError = ""
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.import_dialog_title"),
		Width:      460,
		OnCancelNo: func(win *gui.Window) {
			abortImport(gui.State[State](win))
		},
		CustomView: func(win *gui.Window) gui.View {
			s := gui.State[State](win)
			opts := groupSelectOptions(s, true)
			th := gui.CurrentTheme()
			content := []gui.View{
				gui.Text(gui.TextCfg{
					Text:      i18n.T("bookmark.import_hint"),
					TextStyle: th.TextStyleSecondary,
					Mode:      gui.TextModeWrap,
					Sizing:    gui.FillFit,
				}),
				gui.Select(gui.SelectCfg{
					ID:       "bm-import-group",
					Label:    i18n.T("bookmark.import_target_group"),
					Selected: []string{s.ImportGroup},
					Options:  opts,
					Sizing:   gui.FillFit,
					Disabled: s.ImportBusy,
					OnSelect: func(vals []string, ctx gui.EventCtx) {
						ctx.Consume()
						if len(vals) > 0 {
							gui.State[State](ctx.Window).ImportGroup = vals[0]
						}
					},
				}),
			}
			if s.ImportError != "" {
				content = append(content, gui.Text(gui.TextCfg{
					Text:      s.ImportError,
					TextStyle: th.TextStyleError,
					Mode:      gui.TextModeWrap,
					Sizing:    gui.FillFit,
				}))
			}
			content = append(content, actionsRight(
				gui.Button(gui.ButtonCfg{
					ID: "bm-import-cancel", Variant: gui.ButtonSecondary,
					Label: i18n.T("common.cancel"),
					OnClick: func(ctx gui.EventCtx) {
						ctx.Consume()
						abortImport(gui.State[State](ctx.Window))
						ctx.Window.DialogDismiss()
					},
				}),
				primaryBusyBtn(
					"bm-import-ok",
					i18n.T("bookmark.import_select"),
					i18n.T("common.loading"),
					s.ImportBusy,
					func(ctx gui.EventCtx) {
						ctx.Consume()
						st2 := gui.State[State](ctx.Window)
						if st2.ImportBusy {
							return
						}
						runImport(ctx.Window, d, st2.ImportGroup)
					},
				),
			))
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: content,
			})
		},
	})
}

func runImport(w *gui.Window, d *Deps, groupName string) {
	st := gui.State[State](w)
	st.Busy = true
	st.ImportBusy = true
	st.ImportError = ""
	gen := st.ImportGen
	w.NativeOpenDialog(gui.NativeOpenDialogCfg{
		Title: i18n.T("bookmark.import_dialog_title"),
		OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
			s := gui.State[State](win)
			if s.ImportGen != gen {
				return
			}
			if res.Status != gui.DialogOK || len(res.Paths) == 0 {
				s.Busy = false
				s.ImportBusy = false
				return
			}
			path := res.Paths[0].Path
			go func() {
				result, err := d.Bookmarks.ImportOpenSSH(path, groupName)
				win.QueueCommand(func(w2 *gui.Window) {
					s2 := gui.State[State](w2)
					if s2.ImportGen != gen {
						// User cancelled the dialog; import may still have
						// written rows — refresh the tree without toasts.
						if err == nil {
							reload(w2, d)
						}
						return
					}
					s2.Busy = false
					s2.ImportBusy = false
					if err != nil {
						s2.ImportError = i18n.Tf("bookmark.import_failed", errText(err))
						return
					}
					w2.DialogDismiss()
					reload(w2, d)
					if result.Created > 0 {
						toastOK(w2, i18n.Tf("bookmark.import_success", result.Created))
					} else {
						toastInfo(w2, i18n.T("bookmark.import_none"))
					}
					if len(result.Warnings) > 0 {
						showWarnings(w2, result.Warnings)
					}
				})
			}()
		},
	})
}

func promptExport(w *gui.Window, d *Deps) {
	st := gui.State[State](w)
	st.ExportGroup = ""
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.export_dialog_title"),
		Width:      420,
		CustomView: func(win *gui.Window) gui.View {
			s := gui.State[State](win)
			opts := []gui.SelectOption{{Label: i18n.T("bookmark.export_all"), Value: ""}}
			opts = append(opts, groupSelectOptions(s, false)...)
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: []gui.View{
					gui.Select(gui.SelectCfg{
						ID:       "bm-export-scope",
						Label:    i18n.T("bookmark.export_scope"),
						Selected: []string{s.ExportGroup},
						Options:  opts,
						Sizing:   gui.FillFit,
						OnSelect: func(vals []string, ctx gui.EventCtx) {
							ctx.Consume()
							if len(vals) > 0 {
								gui.State[State](ctx.Window).ExportGroup = vals[0]
							}
						},
					}),
					actionsRight(
						gui.Button(gui.ButtonCfg{
							ID: "bm-export-cancel", Variant: gui.ButtonSecondary,
							Label: i18n.T("common.cancel"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
							},
						}),
						gui.Button(gui.ButtonCfg{
							ID: "bm-export-ok", Variant: gui.ButtonPrimary,
							Label: i18n.T("common.export"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								scope := gui.State[State](ctx.Window).ExportGroup
								ctx.Window.DialogDismiss()
								runExport(ctx.Window, d, scope)
							},
						}),
					),
				},
			})
		},
	})
}

func runExport(w *gui.Window, d *Deps, groupName string) {
	st := gui.State[State](w)
	st.Busy = true
	go func() {
		content, warnings, err := d.Bookmarks.ExportOpenSSH(groupName)
		w.QueueCommand(func(win *gui.Window) {
			s := gui.State[State](win)
			s.Busy = false
			if err != nil {
				toastErrText(win, i18n.Tf("bookmark.export_failed", errText(err)))
				return
			}
			if strings.TrimSpace(content) == "" {
				toastErrText(win, i18n.T("bookmark.export_empty"))
				if len(warnings) > 0 {
					showWarnings(win, warnings)
				}
				return
			}
			if len(warnings) > 0 {
				var b strings.Builder
				for _, wmsg := range warnings {
					b.WriteString("# warning: ")
					b.WriteString(wmsg)
					b.WriteByte('\n')
				}
				b.WriteString(content)
				content = b.String()
			}
			win.NativeSaveDialog(gui.NativeSaveDialogCfg{
				Title:            i18n.T("bookmark.export_dialog_title"),
				DefaultName:      "config",
				ConfirmOverwrite: true,
				OnDone: func(res gui.NativeDialogResult, w2 *gui.Window) {
					if res.Status != gui.DialogOK || len(res.Paths) == 0 {
						return
					}
					path := res.Paths[0].Path
					if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
						toastErrText(w2, i18n.Tf("bookmark.export_failed", err.Error()))
						return
					}
					toastOK(w2, i18n.T("bookmark.export_success"))
				},
			})
		})
	}()
}

func showWarnings(w *gui.Window, warnings []string) {
	body := strings.Join(warnings, "\n")
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("bookmark.warnings_title"),
		Width:      520,
		CustomView: func(*gui.Window) gui.View {
			th := gui.CurrentTheme()
			return gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.PaddingMedium,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						ID:         "bm-warnings-scroll",
						Sizing:     gui.FillFit,
						Scrollable: true,
						MaxHeight:  280,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      body,
								TextStyle: th.TextStyleBody,
								Mode:      gui.TextModeWrap,
								Sizing:    gui.FillFit,
							}),
						},
					}),
					actionsRight(
						gui.Button(gui.ButtonCfg{
							ID: "bm-warnings-ok", Variant: gui.ButtonPrimary,
							Label: i18n.T("common.ok"),
							OnClick: func(ctx gui.EventCtx) {
								ctx.Consume()
								ctx.Window.DialogDismiss()
							},
						}),
					),
				},
			})
		},
	})
}

func groupSelectOptions(st *State, includeImported bool) []gui.SelectOption {
	opts := make([]gui.SelectOption, 0, len(st.Groups)+1)
	seenImported := false
	for _, g := range st.Groups {
		if g == nil {
			continue
		}
		if g.Name == bookmark.ImportedGroupName {
			seenImported = true
		}
		opts = append(opts, gui.SelectOption{
			Label: displayGroupName(g.Name),
			Value: g.Name,
		})
	}
	if includeImported && !seenImported {
		opts = append([]gui.SelectOption{{
			Label: i18n.T("bookmark.group.imported"),
			Value: bookmark.ImportedGroupName,
		}}, opts...)
	}
	return opts
}
