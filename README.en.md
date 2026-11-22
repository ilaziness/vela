# Vela — Cross-platform SSH / SFTP / serial AI GUI client

[English](README.en.md) | [中文](README.md)

[![CI](https://github.com/ilaziness/vela/actions/workflows/release.yml/badge.svg)](https://github.com/ilaziness/vela/actions/workflows/release.yml)
[![Release](https://img.shields.io/github/v/release/ilaziness/vela)](https://github.com/ilaziness/vela/releases/latest)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey)](https://github.com/ilaziness/vela/releases)
[![Go](https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![go-gui](https://img.shields.io/badge/go--gui-native-0A7EA4)](https://github.com/go-gui-org/go-gui)
[![License](https://img.shields.io/github/license/ilaziness/vela)](LICENSE)

A cross-platform SSH / SFTP / serial desktop client with an AI assistant, for developers and operators.

---

## 📖 Overview

**Vela** is a cross-platform desktop client for SSH logins, SFTP file management, local serial consoles, and a sidebar AI assistant. It is built with Go, [go-gui](https://github.com/go-gui-org/go-gui), and [go-term](https://github.com/go-gui-org/go-term), and runs natively on Windows, macOS, and Linux. The UI is available in English, Simplified Chinese, and Traditional Chinese.

---

## ✨ Features

- Support for SSH and serial protocols
- SSH login: password, private key, certificate, 2FA, SSH Agent
- Jump hosts, HTTP / SOCKS proxies, and local / remote / dynamic port forwarding
- In-app key management; master password protects secrets and private keys
- Import and export OpenSSH config
- Multi-tab terminal, in-terminal search, terminal recording and playback
- SSH sessions support Zmodem (rz / sz) file transfer
- Dual-pane SFTP panel
- Bookmark management
- Backup & restore: master password encryption and compression, upload to self-hosted storage, WebDAV, or S3
- AI assistant: local file read/write, web search, remote command execution, MCP and Skills
- Multi-language: English, Simplified Chinese, Traditional Chinese
- Multiple themes available for app and terminal
- Automatic update check, download, and installation

---

## 📸 Screenshots

> Main screens:

### Main window — SSH terminal

![Main](screenshots/main.png)
_Multi-tab SSH terminal_

![Terminal](screenshots/terminal.png)
_Multi-tab SSH terminal_

### SFTP file browser

![SFTP](screenshots/sftp.png)
_File management_

### Bookmarks

![Bookmarks](screenshots/bookmark.png)
_Saved connection settings_

### Command library

![Command library](screenshots/command.png)

### AI assistant

![AI assistant](screenshots/ai.png)

### Settings

![Settings](screenshots/setting.png)

---

## 📦 Download & install

Get a build for your platform from [Releases](https://github.com/ilaziness/vela/releases):

| Platform | Asset | Notes |
| -------- | ----- | ----- |
| Windows | `Vela_*_windows_amd64.zip` | Unzip and run `Vela.exe` |
| Linux | `Vela_*_linux_amd64.tar.gz` | Unzip and run `Vela` |
| macOS | `Vela_*_darwin_arm64.zip` | Unzip to `Vela.app` (see below) |

### macOS

Only **Apple Silicon (arm64)** builds are published. Intel Macs are not supported.

After a browser download, macOS may block the first launch (nothing happens, “unidentified developer”, or “damaged”). Open it as follows:

1. Unzip to get `Vela.app` (do not open it from inside the zip).
2. In Terminal (adjust the path if needed):

```bash
xattr -cr ~/Downloads/Vela.app
open ~/Downloads/Vela.app
```

3. If it is still blocked: in Finder, **Control-click → Open** and confirm, or use **System Settings → Privacy & Security → Open Anyway**.

---

## AI: MCP and Skills

Config lives in the **user data directory** (**Settings → General → User data directory**, default `data` next to the app).

You can also open the files or folders, toggle enablement, and refresh the list under **Settings → AI → MCP / Skills**.

### MCP

The format matches common Cursor / Claude Code `mcp.json`: a top-level `mcpServers` object keyed by server name.

1. Open `mcp.json` in the user data directory (created as `{"mcpServers":{}}` if missing).
2. Add servers under `mcpServers`, save, then click Refresh on the settings page.
3. Toggle enable/disable. Green means reachable; red means enabled but currently unreachable. Only servers that are both enabled and available are given to the AI. MCP tool calls in chat require your approval.

**Fields** (each `mcpServers.<name>` entry)

| Field | Required | Description |
| ----- | -------- | ----------- |
| `command` / `args` / `env` | for stdio | Launch command and args; `env` is an object, e.g. `{"API_KEY":"value"}` |
| `url` / `headers` | for http | Server URL and request headers (e.g. Authorization) |
| `type` | no | `stdio` or `http`; omitted: `command` implies stdio, `url` implies http |
| `enabled` | no | Whether it is on; default on if omitted (the settings toggle writes this field) |

Secrets in `mcp.json` are stored in plaintext. Keep that file private.

**stdio example**

```json
{
  "mcpServers": {
    "example-time": {
      "command": "uvx",
      "args": ["mcp-server-time"],
      "env": {
        "API_KEY": "value"
      }
    }
  }
}
```

**http example**

```json
{
  "mcpServers": {
    "remote-docs": {
      "url": "https://mcp.example.com/mcp",
      "headers": {
        "Authorization": "Bearer your-token"
      }
    }
  }
}
```

### Skills

1. Create a `skills` folder in the user data directory (or use **Open skills directory** on the settings page).
2. One subfolder per skill, with a `SKILL.md` inside, for example:

```
data/skills/my-skill/SKILL.md
```

3. Refresh on the settings page and use the toggles. Enabled skills are loaded by the AI without approval.

**SKILL.md example**

```markdown
---
description: Short description of what this skill does (optional, recommended)
---

# My skill

Detailed instructions for the AI…
```

The folder name is the skill name. After editing files, click Refresh in settings to pick up changes.

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

```
Copyright 2024 Vela Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
