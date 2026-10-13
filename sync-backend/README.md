# Vela Sync Backend

Vela 数据同步服务端，用于存储和管理用户数据的多版本备份。

## 功能特性

- **多用**户支持: 每个用户通过 sync\_id 隔离数据
- **多版本管理**: 保留最多 500 个历史版本，支持版本恢复和删除
- **数据加密**: 客户端加密后传输，服务端仅存储加密数据
- **频率控制**: 限制每分钟最多同步 1 次，防止滥用
- **流式处理**: 支持大文件流式上传下载，低内存占用
- **版本列表分页**: 支持 limit/offset 分页查询历史版本

## 快速开始

### 1. 编译

```bash
cd sync-backend
go build -o sync-backend .
```

### 2. 生成用户凭证

```bash
./sync-backend gen-user
```

输出示例：

```
User created successfully!

Sync ID:  550e8400-e29b-41d4-a716-446655440000
User Key: a1b2c3d4e5f6...

Please save these credentials securely.
The User Key will be used to derive the encryption key.
```

### 3. 启动服务

```bash
./sync-backend server
```

服务默认监听 `0.0.0.0:8080`。

## 配置说明

创建 `config.toml` 配置文件：

### SQLite 配置（默认）

```toml
[server]
host = "0.0.0.0"
port = 8080

[data]
data_dir = "./data/files"
max_versions = 5

[data.database]
type = "sqlite"
db_path = "./data/sync.db"
```

### MySQL 配置

```toml
[server]
host = "0.0.0.0"
port = 8080

[data]
data_dir = "./data/files"
max_versions = 5

[data.database]
type = "mysql"
host = "localhost"
port = 3306
user = "vela"
password = "your_password"
name = "vela_sync"
```

### 配置项说明

| 配置项                      | 默认值              | 说明                      |
| ------------------------ | ---------------- | ----------------------- |
| `server.host`            | "0.0.0.0"        | HTTP 服务监听地址             |
| `server.port`            | 8080             | HTTP 服务监听端口             |
| `data.data_dir`          | "./data/files"   | 数据文件存储目录                |
| `data.max_versions`      | 500              | 每个用户最大版本数               |
| `data.database.type`     | "sqlite"         | 数据库类型: sqlite 或 mysql   |
| `data.database.db_path`  | "./data/sync.db" | SQLite 数据库路径（SQLite 使用） |
| `data.database.host`     | "localhost"      | MySQL 主机（MySQL 使用）      |
| `data.database.port`     | 3306             | MySQL 端口（MySQL 使用）      |
| `data.database.user`     | ""               | MySQL 用户名（MySQL 使用）     |
| `data.database.password` | ""               | MySQL 密码（MySQL 使用）      |
| `data.database.name`     | ""               | MySQL 数据库名（MySQL 使用）    |

## API 接口

### 认证方式

所有 API（除健康检查外）需要在请求头中携带认证信息：

```
X-Sync-ID: <sync_id>
X-User-Key: <user_key>
```

### 接口列表

#### 健康检查

```
GET /health
```

响应：

```json
{
  "status": "ok"
}
```

#### 上传数据

```
POST /upload
Headers:
  X-Sync-ID: <sync_id>
  X-User-Key: <user_key>
  X-File-Hash: <file_hash>
Body: <加密后的数据流>
```

响应：

```json
{
  "version_number": 1,
  "file_size": 1024,
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### 下载数据

```
GET /download?version=<version_number>
Headers:
  X-Sync-ID: <sync_id>
  X-User-Key: <user_key>
```

响应：数据流

Headers:

- `X-Version-Number`: 版本号

#### 获取版本列表

```
GET /versions?limit=<limit>&offset=<offset>
Headers:
  X-Sync-ID: <sync_id>
  X-User-Key: <user_key>
```

参数：

- `limit`: 返回数量限制（0 表示不分页，返回全部）
- `offset`: 偏移量，用于分页

响应：

```json
{
  "versions": [
    {
      "version_number": 2,
      "file_hash": "sha256_hash",
      "file_size": 2048,
      "created_at": "2024-01-02T00:00:00Z"
    },
    {
      "version_number": 1,
      "file_hash": "sha256_hash",
      "file_size": 1024,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### 删除版本

```
DELETE /versions?version=<version_number>
Headers:
  X-Sync-ID: <sync_id>
  X-User-Key: <user_key>
```

响应：

```json
{
  "message": "version deleted successfully"
}
```

## 生产环境部署

### 使用 systemd 管理

创建 `/etc/systemd/system/vela-sync.service`：

```ini
[Unit]
Description=Vela Sync Backend
After=network.target

[Service]
Type=simple
User=vela
WorkingDirectory=/opt/vela-sync
ExecStart=/opt/vela-sync/sync-backend server -config=/opt/vela-sync/config.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable vela-sync
sudo systemctl start vela-sync
```

### 使用 Nginx 反向代理

```nginx
server {
    listen 443 ssl;
    server_name sync.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 大文件上传配置
        client_max_body_size 1G;
        proxy_request_buffering off;
        proxy_buffering off;
    }
}
```

### Docker 部署

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o sync-backend .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/sync-backend .
COPY --from=builder /app/config.example.toml ./config.toml
VOLUME ["/root/data"]
EXPOSE 8080
CMD ["./sync-backend", "server"]
```

构建并运行：

```bash
docker build -t vela-sync .
docker run -d -p 8080:8080 -v $(pwd)/data:/root/data vela-sync
```

## 数据备份

服务端数据包括：

- **SQLite**: 数据库文件 `data/sync.db`
- **MySQL**: 数据库内容由 MySQL 管理
- **数据文件目录**: `data/files/`（所有版本数据）

建议定期备份：

```bash
# SQLite 备份脚本示例
tar czf backup-$(date +%Y%m%d).tar.gz data/

# MySQL 备份示例
mysqldump -u vela -p vela_sync > backup-$(date +%Y%m%d).sql
```

## 安全建议

1. **使用 HTTPS**: 生产环境务必使用 HTTPS 加密传输
2. **防火墙限制**: 仅允许可信 IP 访问同步端口
3. **定期备份**: 定期备份数据库和数据文件
4. **监控告警**: 设置磁盘空间监控，避免存储耗尽

## 故障排查

### 查看日志

```bash
journalctl -u vela-sync -f
```

### 常见问题

1. **上传失败**: 检查 `client_max_body_size` 是否足够大
2. **磁盘空间不足**: 清理旧版本或扩展存储
3. **数据库锁定**: 确保只有一个服务实例访问数据库

