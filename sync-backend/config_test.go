package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", config.Server.Host)
	}

	if config.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", config.Server.Port)
	}

	if config.Data.MaxFiles != 500 {
		t.Errorf("expected max_files 500, got %d", config.Data.MaxFiles)
	}
}

func TestLoadConfig(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// 测试加载不存在的配置（应该创建默认配置）
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if config.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", config.Server.Port)
	}

	// 验证配置文件已创建
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file should be created")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	config := &ServerConfig{
		Server: Server{
			Host: "127.0.0.1",
			Port: 9090,
		},
		Data: Data{
			Database: Database{
				Type:   "sqlite",
				DBPath: "./test.db",
			},
			DataDir:  "./test_files",
			MaxFiles: 3,
		},
	}

	err := SaveConfig(configPath, config)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// 重新加载验证
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", loaded.Server.Port)
	}

	if loaded.Data.MaxFiles != 3 {
		t.Errorf("expected max_files 3, got %d", loaded.Data.MaxFiles)
	}
}
