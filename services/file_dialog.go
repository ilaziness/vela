package services

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/config"
)

func dialogCancelled(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cancel")
}

func suggestedSaveName(remotePath string) string {
	name := path.Base(strings.Trim(remotePath, "/"))
	if name == "" || name == "." || name == "/" {
		return "download"
	}
	return name
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	dl := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dl); err == nil && st.IsDir() {
		return dl
	}
	return home
}

func dialogStartDir(store *config.Store) string {
	if store != nil {
		if dir := store.LastDialogDir(); dir != "" {
			return dir
		}
	}
	return defaultDownloadDir()
}

// rememberDialogPath stores the selected directory, or the parent of a file / save-as path.
func rememberDialogPath(store *config.Store, selected string) {
	if store == nil || selected == "" {
		return
	}
	dir := selected
	if st, err := os.Stat(selected); err != nil || !st.IsDir() {
		dir = filepath.Dir(selected)
	}
	_ = store.SetLastDialogDir(dir)
}

func withDialogStartDir(dlg *application.OpenFileDialogStruct, store *config.Store) *application.OpenFileDialogStruct {
	if dir := dialogStartDir(store); dir != "" {
		return dlg.SetDirectory(dir)
	}
	return dlg
}

// promptOpenFile shows a single-file open dialog.
func promptOpenFile(app *application.App, store *config.Store, title string) (string, error) {
	dlg := withDialogStartDir(
		app.Dialog.OpenFile().
			SetTitle(title).
			CanChooseDirectories(false).
			CanChooseFiles(true),
		store,
	)
	path, err := dlg.PromptForSingleSelection()
	if err == nil && path != "" {
		rememberDialogPath(store, path)
	}
	return path, err
}

// promptOpenFiles shows a multi-file open dialog with an optional filter.
func promptOpenFiles(app *application.App, store *config.Store, title, filterName, filterPattern string) ([]string, error) {
	dlg := withDialogStartDir(
		app.Dialog.OpenFile().
			SetTitle(title).
			CanChooseDirectories(false).
			CanChooseFiles(true),
		store,
	)
	if filterName != "" && filterPattern != "" {
		dlg.AddFilter(filterName, filterPattern)
	}
	paths, err := dlg.PromptForMultipleSelection()
	if err == nil && len(paths) > 0 {
		rememberDialogPath(store, paths[0])
	}
	return paths, err
}

// promptOpenDirectory shows a directory-only open dialog.
func promptOpenDirectory(app *application.App, store *config.Store, title string) (string, error) {
	dlg := withDialogStartDir(
		app.Dialog.OpenFile().
			SetTitle(title).
			CanChooseDirectories(true).
			CanChooseFiles(false).
			CanCreateDirectories(true),
		store,
	)
	path, err := dlg.PromptForSingleSelection()
	if err == nil && path != "" {
		rememberDialogPath(store, path)
	}
	return path, err
}

// promptSaveFile shows a save dialog with a prefilled filename.
// Wails v3 GTK4 Linux dialogs ignore SetFilename (wails#4841); zenity/kdialog
// are tried first on Linux, then Wails.
func promptSaveFile(app *application.App, store *config.Store, title, filename string) (string, error) {
	filename = suggestedSaveName(filename)
	if runtime.GOOS == "linux" {
		p, err := linuxSaveFile(store, title, filename)
		if err == nil {
			if p != "" {
				rememberDialogPath(store, p)
			}
			return p, nil
		}
	}
	opts := application.SaveFileDialogOptions{
		Title:                title,
		Message:              title,
		Filename:             filename,
		CanCreateDirectories: true,
		Directory:            dialogStartDir(store),
	}
	dlg := app.Dialog.SaveFile()
	dlg.SetOptions(&opts)
	path, err := dlg.PromptForSingleSelection()
	if err == nil && path != "" {
		rememberDialogPath(store, path)
	}
	return path, err
}

func linuxSaveFile(store *config.Store, title, filename string) (string, error) {
	initial := filename
	if dir := dialogStartDir(store); dir != "" {
		initial = filepath.Join(dir, filename)
	}
	cmds := [][]string{
		{"zenity", "--file-selection", "--save", "--confirm-overwrite", "--title", title, "--filename", initial},
		{"kdialog", "--title", title, "--getsavefilename", initial},
	}
	var lookErr error
	for _, args := range cmds {
		if _, err := exec.LookPath(args[0]); err != nil {
			lookErr = err
			continue
		}
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return "", nil
		}
		lookErr = err
	}
	return "", lookErr
}
