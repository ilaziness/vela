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

// promptSaveFile shows a save dialog with a prefilled filename.
// Wails v3 GTK4 Linux dialogs ignore SetFilename (wails#4841); zenity/kdialog
// are tried first on Linux, then Wails.
func promptSaveFile(app *application.App, title, filename string) (string, error) {
	filename = suggestedSaveName(filename)
	if runtime.GOOS == "linux" {
		p, err := linuxSaveFile(title, filename)
		if err == nil {
			return p, nil
		}
	}
	opts := application.SaveFileDialogOptions{
		Title:                title,
		Message:              title,
		Filename:             filename,
		CanCreateDirectories: true,
		Directory:            defaultDownloadDir(),
	}
	dlg := app.Dialog.SaveFile()
	dlg.SetOptions(&opts)
	return dlg.PromptForSingleSelection()
}

func promptOpenDirectory(app *application.App, title string) (string, error) {
	dlg := app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true)
	if dir := defaultDownloadDir(); dir != "" {
		dlg = dlg.SetDirectory(dir)
	}
	return dlg.PromptForSingleSelection()
}

func linuxSaveFile(title, filename string) (string, error) {
	initial := filename
	if dir := defaultDownloadDir(); dir != "" {
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
