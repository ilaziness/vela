package sync

import "strings"

// Provider is the backup transport backend.
type Provider string

const (
	ProviderSelfHosted Provider = "selfhosted"
	ProviderWebDAV     Provider = "webdav"
	ProviderS3         Provider = "s3"
)

// SyncConfig is the nested sync/backup configuration.
type SyncConfig struct {
	Provider   Provider         `toml:"provider" json:"provider"`
	SelfHosted SelfHostedConfig `toml:"selfhosted" json:"selfhosted"`
	WebDAV     WebDAVConfig     `toml:"webdav" json:"webdav"`
	S3         S3Config         `toml:"s3" json:"s3"`
}

// SelfHostedConfig holds credentials for the Vela sync-backend.
type SelfHostedConfig struct {
	ServerURL        string `toml:"server_url" json:"serverUrl"`
	SyncID           string `toml:"sync_id" json:"syncId"`
	UserKey          string `toml:"-" json:"userKey,omitempty"`
	UserKeyEncrypted string `toml:"user_key_encrypted,omitempty" json:"userKeyEncrypted,omitempty"`
}

// WebDAVConfig holds user-supplied WebDAV credentials.
type WebDAVConfig struct {
	URL               string `toml:"url" json:"url"`
	Username          string `toml:"username" json:"username"`
	RemotePath        string `toml:"remote_path" json:"remotePath"`
	Password          string `toml:"-" json:"password,omitempty"`
	PasswordEncrypted string `toml:"password_encrypted,omitempty" json:"passwordEncrypted,omitempty"`
}

// S3Config holds S3-compatible object storage credentials.
type S3Config struct {
	Endpoint           string `toml:"endpoint" json:"endpoint"`
	Region             string `toml:"region" json:"region"`
	Bucket             string `toml:"bucket" json:"bucket"`
	Prefix             string `toml:"prefix" json:"prefix"`
	AccessKey          string `toml:"access_key" json:"accessKey"`
	SecretKey          string `toml:"-" json:"secretKey,omitempty"`
	SecretKeyEncrypted string `toml:"secret_key_encrypted,omitempty" json:"secretKeyEncrypted,omitempty"`
}

// IsConfigured reports whether the active provider has enough settings to operate.
func (c *SyncConfig) IsConfigured() bool {
	if c == nil {
		return false
	}
	switch c.Provider {
	case ProviderSelfHosted:
		sh := c.SelfHosted
		return strings.TrimSpace(sh.ServerURL) != "" &&
			strings.TrimSpace(sh.SyncID) != "" &&
			(strings.TrimSpace(sh.UserKey) != "" || strings.TrimSpace(sh.UserKeyEncrypted) != "")
	case ProviderWebDAV:
		w := c.WebDAV
		return strings.TrimSpace(w.URL) != "" &&
			strings.TrimSpace(w.Username) != "" &&
			strings.TrimSpace(w.RemotePath) != "" &&
			(strings.TrimSpace(w.Password) != "" || strings.TrimSpace(w.PasswordEncrypted) != "")
	case ProviderS3:
		s := c.S3
		return strings.TrimSpace(s.Endpoint) != "" &&
			strings.TrimSpace(s.Bucket) != "" &&
			strings.TrimSpace(s.AccessKey) != "" &&
			(strings.TrimSpace(s.SecretKey) != "" || strings.TrimSpace(s.SecretKeyEncrypted) != "")
	default:
		return false
	}
}
