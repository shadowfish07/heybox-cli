# heybox-cli

[![CI](https://github.com/shadowfish07/heybox-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/shadowfish07/heybox-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/shadowfish07/heybox-cli)](https://github.com/shadowfish07/heybox-cli/releases/latest)
[![License](https://img.shields.io/github/license/shadowfish07/heybox-cli)](LICENSE)

一个只读的小黑盒社区搜索命令行工具。支持搜索帖子、话题、用户和游戏，默认输出适合终端阅读的摘要表格，也可以输出稳定 JSON 供脚本使用。

> 小黑盒没有公开、稳定的社区搜索 API。本项目使用当前网页端/客户端的只读接口，接口变化、限流或验证码都可能影响搜索。CLI 不会尝试绕过验证码。

## 安装

### Homebrew（推荐）

macOS 用户可以一条命令安装到 Homebrew 管理的全局 PATH：

```bash
brew install --cask shadowfish07/tap/heybox
```

升级：

```bash
brew upgrade --cask heybox
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
# 全站统一搜索
heybox search "Steam 夏促"

# 只搜索帖子、话题、用户或游戏
heybox search "Steam" --type post
heybox search "Steam" --type topic
heybox search "Steam" --type user
heybox search "Steam" --type game

# JSON 输出
heybox search "Steam" --json

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

## 可选登录态

默认匿名搜索。如果小黑盒要求登录，可以通过环境变量提供已有 Cookie：

```bash
HEYBOX_COOKIE='heybox_id=...; ...' heybox search "关键词"
```

Cookie 不支持命令行参数，避免出现在 shell 历史和进程列表中；CLI 也不会读取浏览器 Cookie 或把 Cookie 输出到日志。建议只在当前 shell 临时设置，不要提交到仓库。

## 输出和错误

- 表格会按终端宽度收缩，窄终端自动切换为分块列表。
- 详情 URL 仅在小黑盒响应提供仍可访问的地址时输出；每条结果始终保留类型和 ID。
- `--json` 输出包含查询元数据、`partial`、`warnings` 和统一的 `results` 数组。
- 全站统一接口受限时，`all` 模式会尝试返回公开的话题/游戏结果，并设置 `partial: true`、在 stderr 输出警告。
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
git tag v0.1.0
git push origin v0.1.0
```

默认测试只使用本地 fixture 和 `httptest`，不会访问小黑盒。手动在线验证：

```bash
go run ./cmd/heybox search "Steam" --type topic --limit 3
go run ./cmd/heybox search "Steam" --type game --limit 3
go run ./cmd/heybox search "Steam" --json --limit 3
```

主要代码分层：

- `internal/api`：HTTP、签名、认证、重试和上游错误分类。
- `internal/search`：分页、降级策略和统一结果模型。
- `internal/output`：终端表格及 JSON。
- `internal/cli`：命令、参数校验和退出码。
