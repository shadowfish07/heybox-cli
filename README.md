# heybox-cli

[![CI](https://github.com/shadowfish07/heybox-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/shadowfish07/heybox-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/shadowfish07/heybox-cli)](https://github.com/shadowfish07/heybox-cli/releases/latest)
[![License](https://img.shields.io/github/license/shadowfish07/heybox-cli)](LICENSE)

一个只读的小黑盒社区命令行工具。支持搜索帖子、话题、用户和游戏，也能读取帖子正文、评论和楼中楼回复；默认输出适合终端阅读的文本，也可以输出稳定 JSON 供脚本使用。

> 小黑盒没有公开、稳定的社区搜索 API。本项目使用当前网页端/客户端的只读接口，接口变化、限流或验证码都可能影响搜索。CLI 不会尝试绕过验证码。

## 安装

### Homebrew（推荐）

macOS 用户可以一条命令安装到 Homebrew 管理的全局 PATH：

```bash
brew install shadowfish07/tap/heybox
```

升级：

```bash
brew upgrade heybox
```

### Go install

已安装 Go 的用户可以直接安装最新版：

```bash
go install github.com/shadowfish07/heybox-cli/cmd/heybox@latest
```

二进制会安装到 `GOBIN`，或默认的 `$(go env GOPATH)/bin`。请确认该目录位于 `PATH`。

### GitHub Release

macOS、Linux 和 Windows 的预编译包及 SHA256 校验文件位于 [GitHub Releases](https://github.com/shadowfish07/heybox-cli/releases/latest)。

### 从源码构建

需要 Go 1.25 或更高版本。

```bash
make build
./bin/heybox --help
```

从源码安装到 `GOBIN`：

```bash
make install
```

查看安装版本：

```bash
heybox --version
```

## 使用

```bash
# 首次登录：默认生成小黑盒 App 扫码二维码
heybox login
heybox auth status

# 全站统一搜索
heybox search "Steam 夏促"

# 只搜索帖子、话题、用户或游戏
heybox search "Steam" --type post
heybox search "Steam" --type topic
heybox search "Steam" --type user
heybox search "Steam" --type game

# JSON 输出
heybox search "Steam" --json

# 查看帖子正文
heybox post 184714599
heybox post 184714599 --json

# 查看评论；默认同时获取每个根楼层的楼中楼回复
heybox comments 184714599
heybox comments 184714599 --json

# 评论排序、分页和有界全量获取
heybox comments 184714599 --sort latest --page 2 --limit 20
heybox comments 184714599 --all --max-pages 20

# 只读根评论，或调整单个楼层的楼中楼获取上限
heybox comments 184714599 --replies=false
heybox comments 184714599 --max-reply-pages 50

# 分页与有界批量获取
heybox search "Steam" --page 2 --limit 10
heybox search "Steam" --all --max-pages 5 --limit 20

# 排序和请求超时
heybox search "Steam" --sort latest --timeout 20s
```

完整参数：

```text
--type all|post|topic|user|game
--sort relevance|latest|hot
--page 1
--limit 20
--all
--max-pages 5
--json
--timeout 15s
```

`--all` 仍受 `--max-pages` 保护，默认最多获取 5 页，最大允许 20 页；各页按顺序请求，页间有短暂延时以降低触发限流的概率。

`comments` 的排序值为 `hot|oldest|latest`。其 `--all` 和 `--max-pages` 控制根评论分页；默认启用的 `--replies` 会继续请求楼中楼，并保留每条回复的 `root_id`、`reply_to_id` 和 `reply_to_author`。每个根楼层默认最多请求 20 页楼中楼，可通过 `--max-reply-pages` 调整到最多 100 页；达到边界时 JSON 会设置 `partial: true` 并返回警告，不会把截断结果冒充完整结果。

## 登录与凭据安全

默认登录方式适合本机终端、SSH 和远程 Agent：

```bash
heybox login
```

CLI 会调用小黑盒当前网页端使用的二维码接口，在交互式终端显示 Unicode 二维码，并始终生成一个权限为 `0600` 的临时 PNG、输出其路径。使用小黑盒 App 扫码并在手机上确认即可；命令结束后临时图片会自动删除。远程 Agent 可以在命令等待期间展示这张 PNG，因此不依赖远端桌面或远端浏览器。

需要验证码、密码、微信或谷歌二次验证时，可以改用官方网页登录：

```bash
heybox login --browser
```

网页模式只在 `127.0.0.1` 的随机端口监听登录回调，并使用随机 `state` 校验回调。它要求完成登录的浏览器能访问运行 CLI 的这台电脑；纯 SSH/远程 Agent 场景优先使用默认二维码模式。浏览器没有自动打开时：

```bash
heybox login --browser --no-browser
```

登录成功后只保存搜索需要的 `heybox_id`、`pkey` 等会话字段，不保存密码、验证码、二维码或谷歌二次验证材料。文件位于系统用户配置目录下的 `heybox-cli/session.json`：macOS 通常是 `~/Library/Application Support/heybox-cli/session.json`，Linux 通常是 `~/.config/heybox-cli/session.json`，Windows 位于用户 AppData。macOS/Linux 的目录权限设为 `0700`、文件权限设为 `0600`；Windows 继承用户 AppData 的访问控制。写入使用同目录临时文件原子替换，并拒绝符号链接目标。

查看状态或删除本地凭据：

```bash
heybox auth status
heybox logout
```

`HEYBOX_COOKIE` 仍可作为临时覆盖，并且优先级高于本地会话文件：

```bash
HEYBOX_COOKIE='heybox_id=...; ...' heybox search "关键词"
```

Cookie 不支持命令行参数，避免出现在进程列表中；CLI 也不会读取其他浏览器 Cookie 或把凭据输出到日志。会话文件等同于登录凭据，请勿复制给他人或提交到仓库。

## 输出和错误

- 表格会按终端宽度收缩，窄终端自动切换为分块列表。
- 详情 URL 仅在小黑盒响应提供仍可访问的地址时输出；每条结果始终保留类型和 ID。
- `--json` 输出包含查询元数据、`partial`、`warnings` 和统一的 `results` 数组。
- `post --json` 返回帖子正文、作者、话题、时间、URL 和互动统计。
- `comments --json` 将楼中楼放在根评论的 `replies` 数组中；回复其他评论时同时保留回复目标 ID 和作者。
- `all` 模式分别请求帖子、用户、话题和游戏并合并结果；任一来源失败时仍返回其他来源，设置 `partial: true` 并在 stderr 输出具体警告。
- `post` 或 `user` 搜索受限时直接失败，不会把部分结果伪装成完整结果。

退出码：

| 代码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 2 | 参数或一般上游错误 |
| 3 | 需要登录或登录态无效 |
| 4 | 验证码或限流 |
| 5 | 上游响应协议不兼容 |
| 6 | 网络失败、超时或取消 |

## 开发与验证

```bash
make check
```

发布由 GitHub Actions 和 GoReleaser 自动完成。推送语义化版本标签即可创建多平台 Release、checksums、构建证明并更新 Homebrew Tap：

```bash
git tag v0.3.0
git push origin v0.3.0
```

默认测试只使用本地 fixture 和 `httptest`，不会访问小黑盒。手动在线验证：

```bash
go run ./cmd/heybox search "Steam" --type topic --limit 3
go run ./cmd/heybox search "Steam" --type game --limit 3
go run ./cmd/heybox search "Steam" --json --limit 3
go run ./cmd/heybox post 184714599 --json
go run ./cmd/heybox comments 184714599 --limit 5 --json
```

主要代码分层：

- `internal/api`：HTTP、签名、认证、重试和上游错误分类。
- `internal/search`：分页、降级策略和统一结果模型。
- `internal/thread`：帖子详情、根评论分页和楼中楼游标分页。
- `internal/output`：终端表格及 JSON。
- `internal/cli`：命令、参数校验和退出码。
