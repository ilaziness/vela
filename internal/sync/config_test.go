package sync

import "testing"

func TestSyncConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   SyncConfig
		expected bool
	}{
		{
			name:     "empty",
			config:   SyncConfig{},
			expected: false,
		},
		{
			name: "selfhosted incomplete",
			config: SyncConfig{
				Provider: ProviderSelfHosted,
				SelfHosted: SelfHostedConfig{
					ServerURL: "http://localhost:8080",
					SyncID:    "id",
				},
			},
			expected: false,
		},
		{
			name: "selfhosted plaintext key",
			config: SyncConfig{
				Provider: ProviderSelfHosted,
				SelfHosted: SelfHostedConfig{
					ServerURL: "http://localhost:8080",
					SyncID:    "id",
					UserKey:   "key",
				},
			},
			expected: true,
		},
		{
			name: "selfhosted encrypted key",
			config: SyncConfig{
				Provider: ProviderSelfHosted,
				SelfHosted: SelfHostedConfig{
					ServerURL:        "http://localhost:8080",
					SyncID:           "id",
					UserKeyEncrypted: "enc",
				},
			},
			expected: true,
		},
		{
			name: "webdav complete",
			config: SyncConfig{
				Provider: ProviderWebDAV,
				WebDAV: WebDAVConfig{
					URL:        "https://dav.example.com",
					Username:   "u",
					RemotePath: "/Vela",
					Password:   "p",
				},
			},
			expected: true,
		},
		{
			name: "webdav missing path",
			config: SyncConfig{
				Provider: ProviderWebDAV,
				WebDAV: WebDAVConfig{
					URL:      "https://dav.example.com",
					Username: "u",
					Password: "p",
				},
			},
			expected: false,
		},
		{
			name: "s3 complete encrypted",
			config: SyncConfig{
				Provider: ProviderS3,
				S3: S3Config{
					Endpoint:           "https://s3.example.com",
					Bucket:             "b",
					AccessKey:          "ak",
					SecretKeyEncrypted: "enc",
				},
			},
			expected: true,
		},
		{
			name: "s3 missing bucket",
			config: SyncConfig{
				Provider: ProviderS3,
				S3: S3Config{
					Endpoint:  "https://s3.example.com",
					AccessKey: "ak",
					SecretKey: "sk",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsConfigured(); got != tt.expected {
				t.Fatalf("IsConfigured()=%v want %v", got, tt.expected)
			}
		})
	}
}
