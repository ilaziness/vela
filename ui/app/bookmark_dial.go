package app

import (
	"errors"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
)

// BookmarkDialDeps wires engines needed to test bookmarks (no session attach).
type BookmarkDialDeps struct {
	Bookmarks   *bookmark.Service
	SSH         *ssh.Manager
	Serial      *serial.Manager
	GlobalProxy func() ssh.GlobalProxy
}

// SetBookmarkDial injects bookmark test-connection dependencies.
func (c *Controller) SetBookmarkDial(d BookmarkDialDeps) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bookmarks = d.Bookmarks
	c.sshMgr = d.SSH
	c.serialMgr = d.Serial
	c.globalProxy = d.GlobalProxy
	if c.globalProxy == nil {
		c.globalProxy = func() ssh.GlobalProxy { return ssh.GlobalProxy{} }
	}
}

// TestBookmark dials using the form draft (PrepareTest restores masked secrets).
// It does not open a session tab. onDone runs on a background goroutine.
func (c *Controller) TestBookmark(b bookmark.Bookmark, onDone func(error)) {
	go func() {
		err := c.testBookmarkSync(b)
		if onDone != nil {
			onDone(err)
		}
	}()
}

func (c *Controller) testBookmarkSync(b bookmark.Bookmark) error {
	c.mu.Lock()
	svc := c.bookmarks
	sshMgr := c.sshMgr
	serialMgr := c.serialMgr
	globalProxy := c.globalProxy
	c.mu.Unlock()
	if svc == nil {
		return errors.New("bookmark dial is not configured")
	}

	proto, ok := termio.ParseProtocol(b.Protocol)
	if !ok {
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
	switch proto {
	case termio.ProtocolSSH:
		if sshMgr == nil || globalProxy == nil {
			return errors.New("bookmark dial is not configured")
		}
		if err := bookmark.ValidateProxy(b); err != nil {
			return err
		}
		prep, err := svc.PrepareTest(b)
		if err != nil {
			return err
		}
		hops, err := svc.ResolveHops(prep.Endpoint, prep.JumpID)
		if err != nil {
			return err
		}
		proxy, err := ssh.ResolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
			Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
			User: b.ProxyUser, Password: prep.ProxyPassword,
		}, globalProxy())
		if err != nil {
			return ssh.MapUserError(err)
		}
		return ssh.MapUserError(sshMgr.TestConnect(hops, proxy))
	case termio.ProtocolSerial:
		if serialMgr == nil {
			return errors.New("bookmark dial is not configured")
		}
		return serialMgr.TestConnect(serial.OpenConfig{
			Device:      b.SerialDevice,
			BaudRate:    b.BaudRate,
			DataBits:    b.DataBits,
			Parity:      b.Parity,
			StopBits:    b.StopBits,
			FlowControl: b.FlowControl,
			Charset:     b.Charset,
		})
	default:
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
}
