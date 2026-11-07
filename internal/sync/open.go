package sync

import (
	"fmt"

	"github.com/ilaziness/vela/internal/sync/store"
)

// OpenStore builds a transport store from a config whose secret fields are already plaintext.
func OpenStore(cfg *SyncConfig) (store.Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sync config required")
	}
	switch cfg.Provider {
	case ProviderSelfHosted:
		return store.NewSelfHosted(store.SelfHostedOptions{
			ServerURL: cfg.SelfHosted.ServerURL,
			SyncID:    cfg.SelfHosted.SyncID,
			UserKey:   cfg.SelfHosted.UserKey,
		})
	case ProviderWebDAV:
		return store.NewWebDAV(store.WebDAVOptions{
			URL:        cfg.WebDAV.URL,
			Username:   cfg.WebDAV.Username,
			Password:   cfg.WebDAV.Password,
			RemotePath: cfg.WebDAV.RemotePath,
		})
	case ProviderS3:
		return store.NewS3(store.S3Options{
			Endpoint:  cfg.S3.Endpoint,
			Region:    cfg.S3.Region,
			Bucket:    cfg.S3.Bucket,
			Prefix:    cfg.S3.Prefix,
			AccessKey: cfg.S3.AccessKey,
			SecretKey: cfg.S3.SecretKey,
		})
	default:
		return nil, fmt.Errorf("unsupported sync provider: %s", cfg.Provider)
	}
}
