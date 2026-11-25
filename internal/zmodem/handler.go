package zmodem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	zm "github.com/xx25/go-zmodem"

	"github.com/ilaziness/vela/internal/utils"
)

type fileHandler struct {
	f           *Filter
	dir         Direction
	saveDir     string
	offer       *zm.FileOffer
	offerSent   bool
	completedOK bool

	lastAt   time.Time
	lastPct  int
	lastName string
}

func (h *fileHandler) NextFile() *zm.FileOffer {
	if h.dir != DirectionSend || h.offer == nil || h.offerSent {
		return nil
	}
	h.offerSent = true
	return h.offer
}

func (h *fileHandler) AcceptFile(info zm.FileInfo) (io.WriteCloser, int64, error) {
	if h.dir != DirectionReceive || h.saveDir == "" {
		return nil, 0, zm.ErrSkip
	}
	name := zm.SanitizeFilename(info.Name)
	if name == "" || name == "." || name == ".." {
		return nil, 0, zm.ErrSkip
	}
	path := filepath.Join(h.saveDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, 0, err
	}
	h.f.emitLine(fmt.Sprintf("Zmodem: receiving %s", name))
	return f, 0, nil
}

func (h *fileHandler) FileProgress(info zm.FileInfo, bytesTransferred int64) {
	name := zm.SanitizeFilename(info.Name)
	if name == "" {
		name = info.Name
	}
	total := info.Size
	pct := 0
	if total > 0 {
		pct = int(bytesTransferred * 100 / total)
		if pct > 100 {
			pct = 100
		}
	}

	now := time.Now()
	if name == h.lastName && pct == h.lastPct && now.Sub(h.lastAt) < 300*time.Millisecond {
		return
	}
	h.lastAt = now
	h.lastPct = pct
	h.lastName = name

	verb := "receiving"
	if h.dir == DirectionSend {
		verb = "sending"
	}
	var msg string
	if total > 0 {
		msg = fmt.Sprintf("Zmodem: %s %s %d%% (%s/%s)",
			verb, name, pct, utils.FormatBytes(bytesTransferred), utils.FormatBytes(total))
	} else {
		msg = fmt.Sprintf("Zmodem: %s %s %s", verb, name, utils.FormatBytes(bytesTransferred))
	}
	h.f.emitProgress(msg)
}

func (h *fileHandler) FileCompleted(info zm.FileInfo, bytesTransferred int64, err error) {
	name := zm.SanitizeFilename(info.Name)
	if name == "" {
		name = info.Name
	}
	if err != nil {
		h.f.emitLine(fmt.Sprintf("Zmodem: %s failed: %v", name, err))
		return
	}
	h.completedOK = true
	h.f.emitLine(fmt.Sprintf("Zmodem: %s complete (%s)", name, utils.FormatBytes(bytesTransferred)))
}

func openFileOffer(path string) (*zm.FileOffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	mode := uint32(st.Mode().Perm())
	return &zm.FileOffer{
		Name:    filepath.Base(path),
		Size:    st.Size(),
		ModTime: st.ModTime(),
		Mode:    mode,
		Reader:  f,
	}, nil
}
