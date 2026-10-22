package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/buildinfo"
	"github.com/ilaziness/vela/internal/httpproxy"
	"github.com/ilaziness/vela/internal/system"
	"github.com/ilaziness/vela/internal/termws"
)

const (
	EventNewVersion   = "eventNewVersion"
	githubRepo        = "ilaziness/vela"
	checksumAssetName = "SHA256SUMS"
)

func init() {
	application.RegisterEvent[NewVersion](EventNewVersion)
}

type AppInfo struct {
	Version   string
	HomeURL   string
	RunMode   string
	GitInfo   string
	BuildTime string
}

type NewVersion struct {
	Version string
	Notes   string
	URL     string
}

type AppService struct {
	app         *application.App
	windows     *Windows
	termWS      *termws.Server
	mainWindow  *application.WebviewWindow
	logger      *zap.Logger
	keybindings *KeyBindingManager
	updateMu    sync.Mutex
}

func NewAppService(app *application.App, windows *Windows, termWS *termws.Server, logger *zap.Logger) *AppService {
	if logger == nil {
		logger = zap.NewNop()
	}
	cs := &AppService{app: app, windows: windows, termWS: termWS, mainWindow: windows.Main, logger: logger}
	cs.initUpdater()
	cs.cleanupUpdateLeftovers()
	return cs
}

func (cs *AppService) initUpdater() {
	gh, err := github.New(github.Config{
		Repository:    githubRepo,
		ChecksumAsset: checksumAssetName,
		HTTPClient:    httpproxy.Client(),
	})
	if err != nil {
		cs.logger.Error("github updater provider", zap.Error(err))
		return
	}
	if err := cs.app.Updater.Init(updater.Config{
		CurrentVersion: trimVersionPrefix(buildinfo.Version),
		Providers:      []updater.Provider{gh},
		Window:         updater.WindowNone,
	}); err != nil {
		cs.logger.Error("updater init", zap.Error(err))
	}
}

// cleanupUpdateLeftovers removes Windows rename-aside leftovers next to the
// running binary and orphaned wails-update staging dirs / logs under TempDir.
func (cs *AppService) cleanupUpdateLeftovers() {
	if exe, err := os.Executable(); err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			exe = resolved
		}
		matches, globErr := filepath.Glob(exe + ".old.*")
		if globErr != nil {
			cs.logger.Debug("glob update leftovers", zap.Error(globErr))
		} else {
			for _, m := range matches {
				if removeErr := os.Remove(m); removeErr != nil {
					cs.logger.Debug("remove update leftover", zap.String("path", m), zap.Error(removeErr))
				}
			}
		}
	}

	tmp := os.TempDir()
	entries, err := os.ReadDir(tmp)
	if err != nil {
		cs.logger.Debug("read temp dir for updater cleanup", zap.Error(err))
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "wails-update-") {
			continue
		}
		path := filepath.Join(tmp, name)
		if removeErr := os.RemoveAll(path); removeErr != nil {
			cs.logger.Debug("remove updater temp", zap.String("path", path), zap.Error(removeErr))
		}
	}
}

func (cs *AppService) MainWindowMin() {
	cs.mainWindow.Minimise()
}
func (cs *AppService) MainWindowMax() {
	if cs.mainWindow.IsMaximised() {
		cs.mainWindow.UnMaximise()
		return
	}
	cs.mainWindow.Maximise()
}
func (cs *AppService) MainWindowClose() {
	cs.mainWindow.Close()
}

func (cs *AppService) SelectDirectory() (string, error) {
	return cs.app.Dialog.OpenFile().SetTitle("选择目录").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}

func (cs *AppService) SelectFile() (string, error) {
	return cs.app.Dialog.OpenFile().SetTitle("选择文件").
		CanChooseDirectories(false).
		CanChooseFiles(true).
		PromptForSingleSelection()
}

func (cs *AppService) GetAppInfo() AppInfo {
	return AppInfo{
		Version:   buildinfo.Version,
		HomeURL:   "https://github.com/ilaziness/vela",
		RunMode:   buildinfo.Mode,
		GitInfo:   buildinfo.GitInfo,
		BuildTime: buildinfo.BuildTime,
	}
}

func (cs *AppService) ListKeyBindings() []KeyBindingInfo {
	if cs.keybindings == nil {
		return nil
	}
	return cs.keybindings.List()
}

func (cs *AppService) SetKeyBinding(action, keys string) error {
	if cs.keybindings == nil {
		return fmt.Errorf("快捷键管理器未初始化")
	}
	return cs.keybindings.Set(action, keys)
}

func (cs *AppService) ResetAllKeyBindings() error {
	if cs.keybindings == nil {
		return fmt.Errorf("快捷键管理器未初始化")
	}
	return cs.keybindings.ResetAll()
}

func (cs *AppService) SuspendKeyBindings() {
	if cs.keybindings != nil {
		cs.keybindings.Suspend()
	}
}

func (cs *AppService) ResumeKeyBindings() {
	if cs.keybindings != nil {
		cs.keybindings.Resume()
	}
}

func (cs *AppService) CheckUpdate() (hasNew bool, newVersion NewVersion, err error) {
	cs.updateMu.Lock()
	defer cs.updateMu.Unlock()

	rel, err := cs.app.Updater.Check(context.Background())
	if err != nil {
		cs.logger.Error("check update failed", zap.Error(err))
		return false, NewVersion{}, err
	}
	if rel == nil {
		return false, NewVersion{}, nil
	}
	return true, newVersionFromRelease(rel), nil
}

// InstallUpdate re-checks for a release, downloads and verifies it, then restarts
// into the new binary. Re-check makes the call safe if a prior Check was skipped
// or its pending state was overwritten.
func (cs *AppService) InstallUpdate() error {
	cs.updateMu.Lock()
	defer cs.updateMu.Unlock()

	ctx := context.Background()
	rel, err := cs.app.Updater.Check(ctx)
	if err != nil {
		cs.logger.Error("install update check failed", zap.Error(err))
		return err
	}
	if rel == nil {
		err := fmt.Errorf("当前没有可用更新")
		cs.logger.Error("install update failed", zap.Error(err))
		return err
	}
	if err := cs.app.Updater.DownloadAndInstall(ctx); err != nil {
		cs.logger.Error("download and install update failed", zap.Error(err))
		return fmt.Errorf("下载或安装更新失败: %w", err)
	}
	if err := cs.app.Updater.Restart(ctx); err != nil {
		cs.logger.Error("restart after update failed", zap.Error(err))
		return fmt.Errorf("重启应用失败: %w", err)
	}
	return nil
}

// startBackgroundUpdateCheck waits 10s for the frontend to initialize, then
// silently checks for updates and emits EventNewVersion when a newer release exists.
func (cs *AppService) startBackgroundUpdateCheck() {
	system.SafeGo(func() {
		time.Sleep(10 * time.Second)
		cs.updateMu.Lock()
		defer cs.updateMu.Unlock()

		rel, err := cs.app.Updater.Check(context.Background())
		if err != nil {
			cs.logger.Error("background update check failed", zap.Error(err))
			return
		}
		if rel == nil {
			return
		}
		cs.app.Event.Emit(EventNewVersion, newVersionFromRelease(rel))
	})
}

func newVersionFromRelease(rel *updater.Release) NewVersion {
	if rel == nil {
		return NewVersion{}
	}
	nv := NewVersion{
		Version: rel.Version,
		Notes:   rel.Notes,
	}
	if rel.Metadata != nil {
		if tag, ok := rel.Metadata["github.release.tag"].(string); ok && tag != "" {
			nv.Version = tag
		}
		if url, ok := rel.Metadata["github.release.htmlURL"].(string); ok {
			nv.URL = url
		}
	}
	if nv.Version != "" && !strings.HasPrefix(nv.Version, "v") && !strings.HasPrefix(nv.Version, "V") {
		nv.Version = "v" + nv.Version
	}
	return nv
}

func trimVersionPrefix(v string) string {
	return strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
}

func (cs *AppService) GetWSAddr() string {
	if cs.termWS == nil {
		return ""
	}
	return cs.termWS.Addr()
}

func (cs *AppService) NewMainWindow() {
	cs.windows.NewMainWindow()
}
