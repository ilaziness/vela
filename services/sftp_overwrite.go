package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
)

const overwriteNameLimit = 5

func overwriteConfirmMessage(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return i18n.Tf("sftp.confirm_overwrite_one", names[0])
	}
	shown := names
	suffix := ""
	if len(shown) > overwriteNameLimit {
		shown = shown[:overwriteNameLimit]
		suffix = "…"
	}
	return i18n.Tf("sftp.confirm_overwrite_many", len(names), strings.Join(shown, ", ")+suffix)
}

// confirmOverwrite returns true when there are no conflicts or the user accepts overwrite.
func (s *SftpService) confirmOverwrite(names []string) bool {
	if len(names) == 0 {
		return true
	}
	return s.askYes(i18n.T("sftp.confirm_overwrite_title"), overwriteConfirmMessage(names))
}

func (s *SftpService) confirmConflicts(names []string, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	return s.confirmOverwrite(names), nil
}

// confirmLocalPathOverwrite returns false when the user declines overwriting an existing path.
func (s *SftpService) confirmLocalPathOverwrite(localPath string) (bool, error) {
	_, err := os.Stat(localPath)
	if err == nil {
		return s.confirmOverwrite([]string{filepath.Base(localPath)}), nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}

// askYes shows a native Yes/No question dialog. Cancel / close counts as No.
func (s *SftpService) askYes(title, message string) bool {
	if s.app == nil {
		s.logger.Error("confirm dialog unavailable", zap.String("message", message))
		return false
	}
	result := make(chan bool, 1)
	send := func(yes bool) {
		select {
		case result <- yes:
		default:
		}
	}
	dlg := s.app.Dialog.Question().SetTitle(title).SetMessage(message)
	yes := dlg.AddButton(i18n.T("dialog.yes"))
	no := dlg.AddButton(i18n.T("dialog.no"))
	yes.OnClick(func() { send(true) })
	no.OnClick(func() { send(false) })
	if runtime.GOOS == "windows" {
		cancel := dlg.AddButton(i18n.T("dialog.cancel"))
		cancel.OnClick(func() { send(false) })
	}
	dlg.SetDefaultButton(no)
	dlg.SetCancelButton(no)
	dlg.Show()
	if runtime.GOOS == "windows" {
		select {
		case yes := <-result:
			return yes
		default:
			return false
		}
	}
	return <-result
}
