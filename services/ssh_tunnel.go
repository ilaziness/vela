package services

import "github.com/ilaziness/vela/internal/tunnel"

type TunnelList = tunnel.TunnelList
type TunnelInfo = tunnel.TunnelInfo

type SSHTunnelService struct {
	mgr *tunnel.Manager
}

func NewSSHTunnelService(mgr *tunnel.Manager) *SSHTunnelService {
	return &SSHTunnelService{mgr: mgr}
}

func (t *SSHTunnelService) TunnelList() []TunnelList {
	return t.mgr.List()
}

func (t *SSHTunnelService) StartLocal(sessionID, localAddr, remoteAddr string) (string, error) {
	return t.mgr.StartLocal(sessionID, localAddr, remoteAddr)
}
func (t *SSHTunnelService) StartRemote(sessionID string, remotePort int, localAddr string) (string, error) {
	return t.mgr.StartRemote(sessionID, remotePort, localAddr)
}
func (t *SSHTunnelService) StartDynamic(sessionID, localAddr string) (string, error) {
	return t.mgr.StartDynamic(sessionID, localAddr)
}
func (t *SSHTunnelService) StopByID(tunnelID string) error {
	return t.mgr.StopByID(tunnelID)
}
func (t *SSHTunnelService) StopAll() {
	t.mgr.StopAll()
}
func (t *SSHTunnelService) StopAllBySession(sessionID string) {
	t.mgr.StopAllBySession(sessionID)
}
