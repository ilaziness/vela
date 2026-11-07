package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	Server Server `toml:"server"`
	Data   Data   `toml:"data"`
}

// Server HTTP服务器配置
type Server struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// Database 数据库配置
type Database struct {
	Type     string `toml:"type"`     // 数据库类型: sqlite 或 mysql
	Host     string `toml:"host"`     // MySQL 主机
	Port     int    `toml:"port"`     // MySQL 端口
	User     string `toml:"user"`     // MySQL 用户名
	Password string `toml:"password"` // MySQL 密码
	Name     string `toml:"name"`     // MySQL 数据库名
	DBPath   string `toml:"db_path"`  // SQLite 数据库路径
}

// Data 数据存储配置
type Data struct {
	Database Database `toml:"database"`  // 数据库配置
	DataDir  string   `toml:"data_dir"`  // 数据文件存储目录
	MaxFiles int      `toml:"max_files"` // 每个用户最多保留的备份文件数
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Server: Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Data: Data{
			Database: Database{
				Type:   "sqlite",
				DBPath: "./data/sync.db",
			},
			DataDir:  "./data/files",
			MaxFiles: 500,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*ServerConfig, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 配置文件不存在，创建默认配置
			return config, SaveConfig(path, config)
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// 确保数据目录存在
	if config.Data.Database.Type == "sqlite" {
		if err := os.MkdirAll(filepath.Dir(config.Data.Database.DBPath), 0755); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(config.Data.DataDir, 0755); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(path string, config *ServerConfig) error {
	data, err := toml.Marshal(config)
	if err != nil {
		return err
	}

	path = filepath.Clean(path)
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
