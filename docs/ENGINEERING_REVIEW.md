# MSP 工程化审阅

日期：2026-08-13  
范围：构建/发布、后端架构、前端工程化、安全与文档

## 核实与落地（同日）

对照代码逐条核实后动手，避免误判：

| 条目 | 核实 | 处理 |
|------|------|------|
| 设置保存清 PIN | 实锤：UI POST SafeConfig，无 `pinHash`，`*c = cfg` + ApplyDefaults 关 PIN | 已修：写入时保留已有 hash；响应改回 SafeConfig |
| 缩略图无分享校验 | 实锤：只 DecodeID + Stat | 已修：走 `resolveMediaTarget` |
| `msp.key` 失败降级 base64 | 实锤 | 已修：启动失败即退出 |
| folder playlist 当 ID 是路径 | 实锤：`absPathOfItem` | 已修：`playlistFolderKey(relPath)` |
| WriteTimeout 60s | 实锤 | 已修：改为 0 |
| Docker CMD | 实锤 | 已修：`/opt/msp-server` + HEALTHCHECK |
| release 文件名重复 suffix | 实锤 | 已修：显式 `artifact` |
| tag 不跑测试 / notes 可缺 | 实锤 | 已修：`workflow_call` check；缺 notes 失败 |
| Validate 未接入生产 | 实锤 | 已修：UpdateConfig / 热重载 / 加载 |
| Config() 浅拷贝竞态 | 实锤 | 已修：Clone + add-share copy-on-write |
| SQLite PRAGMA 仅首连接 | 实锤 | 已修：写入 DSN；池上限 4 |
| PIN 计数锁外自增 | 实锤 | 已修 |
| HLS `list_size 0` | **误判**：点播 seek 需要完整播放列表，不改 |
| 改 CF 头默认行为 | **不改**：会弄坏现有 Tunnel + 白名单；只改文档 |

以下为原始审阅正文。

MSP 已经是同体量个人项目里工程化做得比较完整的一档：单二进制、跨平台 CI、bcrypt PIN、路径 TOCTOU、优雅关停、DB 降级、文档和发布说明都在。下面按「现在会痛 / 规模化会痛 / 卫生债」写，不重复 `docs/archive/` 里已经修掉的旧账（明文 PIN、无 PR CI、RateLimiter 无上限、随机 nonce MediaID 等）。

---

## 1. 总评

| 维度 | 现状 | 短板 |
|------|------|------|
| 构建 / 发布 | 多架构矩阵、checksum、frozen-lockfile | 产物文件名错误；tag 不跑测试；二进制无版本；无镜像发布 |
| 后端结构 | 接口注入、编译期断言、分层意识清楚 | `handler` / `media` 过重；双缓存；死包 |
| 安全 | PIN 哈希、Admin 本机锁、路径校验 | 设置保存会清 PIN；缩略图未校验分享目录；文档过时 |
| 测试 | Go 测试面宽，含 race | 覆盖率掺水；前端零测试；无漏洞扫描 |
| 前端 | 模块拆分、PWA、keyed diff | 无 lint；第三方库手拷；folder scope 回归 |
| 文档 | CodeMap / API / 转码 / 发布说明齐全 | SECURITY / CONTRIBUTING 与代码漂移 |

定位是「家庭局域网影院」，很多「不安全」是有意为之。工程化上真正该补的，是**门禁不对称、运行时契约不稳、文档会把人配错、前端没有质量闸**。

不要为了「更像大厂」去加 OpenTelemetry、完整 Clean Architecture、TypeScript 重写。优势是单文件分发和少依赖；工程化应保护这条路径。

---

## 2. 已经值得保留的做法

- **嵌入式前端契约明确**：`web/embed.go` 强制先 build；CI 的 lint job 用 dummy `web/dist`，test/build 才真编前端。
- **依赖注入落地**：`cmd/msp/main.go` 对 `ConfigProvider` / `SessionProvider` / store 做了编译期断言。
- **安全细节到位**：`resolveMediaTarget` 在 `Open` 后再 `EvalSymlinks` + `IsAllowedFile`；PIN 走 bcrypt；session 用 `crypto/rand`；管理 API 本机锁。
- **故障隔离**：DB 起不来只禁用进度/收藏/偏好，并打横幅；`storage.ErrUnavailable` 映射 503。
- **关停顺序正确**：先杀转码进程 → `Shutdown` → 取消后台扫描并等落盘 → 关 DB / 日志。
- **SQLite 调参务实**：WAL、`busy_timeout`、`mmap`、`PrepareStmt`。
- **前端已拆模块**：`player/`、`playlist/`、`ui/` + eventbus；列表 keyed diff + 事件委托。
- **API 客户端一致**：`credentials: "include"`、JSON 校验、503 `db_unavailable` 降级。
- **设置 UI 仅本机可见**（`accessLevel === 'local'`）。
- **PWA** 对 `/api/` 禁止 navigate fallback，接口默认 `NetworkOnly`。

---

## 3. 建议落地顺序

按投入产出，不是按教科书完整度。

### 先修（可复现缺陷）

1. 配置写入合并，禁止 SafeConfig 覆盖 `pinHash`
2. 缩略图走 `resolveMediaTarget`；`msp.key` 失败则退出
3. 播放列表 folder scope 改用 `relPath`
4. 流式接口去掉全局 60s `WriteTimeout`
5. Docker `CMD` 改为 `/opt/msp-server` + `HEALTHCHECK`
6. 统一 release 产物文件名；tag 必须过测试；缺 `docs/release/<tag>.md` 则失败
7. `ldflags` 注入版本；`/healthz` 带出版本
8. 同步 SECURITY / CONTRIBUTING / Dependabot 注释

### 一周内（小、可验证）

9. 删 `internal/types` 和未使用的 HTTP 状态码常量
10. `.gitignore` 补 `data/`、`media/`、`thumbs/`
11. `config.Validate` 接入加载 / POST / 热重载
12. SQLite PRAGMA 写入 DSN；降低 `MaxOpenConns`

### 一个月内

13. CI 加 `bun run lint` + `govulncheck`
14. 给 `sort-filter.js`、`ui/diff.js`、PIN 启动路径补最小前端测试
15. `trustProxy` / Cloudflare 头改为显式开关
16. PIN 开启时收紧 `GET /api/config`
17. HLS 改滑动窗口，限制临时盘占用
18. 停止 SW 缓存 `/api/thumbnail`（或改为 `private` + `NetworkFirst`）
19. SVG 改为 `attachment` 或不进图片页

### 一个季度（库变大再做）

20. 去掉 JSON 全量缓存，SQLite 做唯一索引
21. 版本化 SQL 迁移
22. plyr / hls.js / pinyin-pro 改 npm
23. 发布打 GHCR 镜像
24. `storage.MediaStore` 接口，停止把 `*gorm.DB` 传给 `media`

---

## 4. P1：现在就会踩的缺陷

### 4.1 设置页「保存过滤」会清掉 PIN

`web/src/modules/ui/bindings.js` 把 `state.config`（SafeConfig，没有 `pinHash`）整包 POST 回 `/api/config`。服务端 `service/config.go` 执行 `*c = cfg`，再 `ApplyDefaults`：哈希和明文都空时强制 `pinEnabled: false` 并落盘。

本机点一次「保存过滤」，局域网 PIN 就没了。`config.Validate()` 在这条路径上从未调用。

**建议**：合并写入，或单独 `PATCH /api/blacklist`。绝不让 SafeConfig 覆盖 `Security`。保留已有 `PINHash`，除非请求里带了新的明文 `pin`。

### 4.2 `/api/thumbnail` 未校验分享目录

`HandleThumbnail` 只 `DecodeID` + `Stat`，不像 `resolveMediaTarget` 那样 `IsAllowedFile` + 二次 `EvalSymlinks`。

叠加：`msp.key` 创建失败只打 Warn（`cmd/msp/main.go`），`EncodeID` / `DecodeID` 退化成路径的 base64。安装目录不可写时，这就是线上模式——能打到 `/api/thumbnail` 就能让 ffmpeg 碰任意本地文件。分享移除后，旧 ID 仍能出缩略图。

**建议**：缩略图复用 `resolveMediaTarget`。`LoadOrCreateKey` 失败则 `log.Fatal`，不要静默降级。

### 4.3 文件夹范围播放列表在 ID 加密后坏了

`web/src/modules/utils.js` 的 `absPathOfItem` 仍把 `item.id` 当路径的 base64 解。ID 已是 AES-GCM，解出来是垃圾。`playlist/navigation.js` 默认 `scope: "folder"` 经常只剩当前这一条。

文件夹浏览已经用 `relPath`（`folder.js`），播放列表应同样改，并补一条单测。

### 4.4 `WriteTimeout: 60s` 会掐断长视频流

```go
// cmd/msp/main.go
WriteTimeout: 60 * time.Second,
```

Go 的 `WriteTimeout` 覆盖整段响应写出。直连大文件、渐进式转码、长 audiobook 都可能在 60 秒被掐断。浏览器常用 Range，所以日常可能「看起来没事」。HLS 分段（4s）不受影响。v1.5.0 加这个超时是防慢客户端，对媒体服务器方向反了。

**建议**：`WriteTimeout` 置 `0`，保留 `ReadHeaderTimeout`；或对 `/api/stream`、`/api/hls/` 用 `http.NewResponseController` 清 write deadline。

### 4.5 Docker 裸跑失败，compose 在打补丁

镜像把二进制放到 `/opt/msp-server`，`CMD` 却是 `./msp-server`。`docker run` 会立刻失败。compose 用一段 shell 把二进制拷进数据卷再 `exec`。

另外：进程以 **root** 跑，无 `USER`、无 `HEALTHCHECK`（虽然有 `/healthz`）、release 不发镜像。v1.12.0 因写权限拿掉了 `appuser`。compose 播种的配置没有 PIN。

**建议**：

- `CMD ["/opt/msp-server"]`，数据目录用环境变量或 `--data-dir`，不要靠「exe 目录 = 数据卷」。
- 加 `HEALTHCHECK`；compose 加 `init: true`。
- 国内镜像源改为 `ARG`，避免海外构建直接失败。
- 发布流水线顺带 `docker buildx` + GHCR。

### 4.6 Release 产物文件名错误

`release.yml` 把 `goarch` 和 `suffix` 拼在一起：

| 实际发出 | 脚本 / 文档期望 |
|----------|----------------|
| `msp-linux-arm-armv7` | `msp-linux-armv7` |
| `msp-linux-loong64-loong64` | `msp-linux-loong64` |
| `msp-windows-386-386.exe` | `msp-windows-386.exe` |

checksum 文件名跟着错。应在 matrix 里显式写文件名，与 `scripts/README.md` 对齐。

### 4.7 打 tag 不会跑测试

`check.yml` 的 `on.push.branches: ["**"]` 不包含 tag。`release.yml` 只编译、不测试不 lint。`git tag && git push --tags` 可以发出未经门禁的提交。

tag push 时若缺 `docs/release/$TAG.md`，会降级成 *See commit history* 然后继续发版。`AGENTS.md` 把这件事列为常见事故，CI 却抓不住。

**建议**：release `needs` 测试任务（或 `workflow_call` 复用 check）；缺 notes 文件则 `exit 1`。

### 4.8 二进制没有版本

本地和 CI 都是 `go build -trimpath -ldflags="-s -w"`，没有 `-X`。出了问题用户只能报「我下的那个 exe」。

**建议**：注入 `version` / `commit` / `date`；`/healthz` 和启动横幅带上。release 用 `github.ref_name`。

### 4.9 安全文档还在教明文 PIN

`config.example.json` 已是 `pinHash: ""`。代码里 `PIN` 只是写入时的瞬时字段，会 bcrypt 后清空。但 `docs/SECURITY.md` 仍写：

- 配置示例用 `"pin": "1234"`
- 「默认为 `"0000"`」
- 「删掉 config.json 后 PIN 变回 0000」

实际默认是 **PIN 关闭、空哈希**。按文档操作的人会以为自己有默认口令。

`CONTRIBUTING.md` 还要求 Node.js，并写 `go build ./cmd/msp`（没有先编前端）。Dependabot 注释写的是 pnpm，实际是 bun。

### 4.10 `config.Validate` 只在测试里跑

加载、热重载、`POST /api/config` 都只 `ApplyDefaults` + `SanitizeSecurity`。手改 `port: 99999` 或非法 `hwAccel` 会直接生效，热重载也没有回滚。

**建议**：三处提交前都 `Validate`；失败则保留旧配置并打日志。

### 4.11 `Config()` 浅拷贝 + `append(Shares)` 竞态

`Server.Config()` 返回的 struct 里 slice 与内部配置共享底层数组。`handleShareAdd` 原地 `append` 时，并发的媒体请求可能读到被改写的 shares。IP 列表、黑名单同理。

**建议**：读时拷贝 slice，或把配置当不可变值整体替换。

### 4.12 SQLite PRAGMA 只打在第一条连接上

WAL 是文件级，`busy_timeout` / `cache_size` / `temp_store` / `mmap_size` 是连接级。连接池扩容后新连接没有 `busy_timeout`。`MaxOpenConns = GOMAXPROCS`，大库扫描事务持锁时，进度写入容易 `SQLITE_BUSY`。

**建议**：PRAGMA 写进 DSN；`MaxOpenConns` 降到 4 左右。在测试里用第二条连接断言 `busy_timeout`。

### 4.13 HLS 把整部片子写进临时盘

`-hls_list_size 0` 保留全部分段，无磁盘上限，会话 5 分钟无访问才清。一部大片就能把 `$TMP/msp_hls_*` 写满。janitor goroutine 没有关停入口。

**建议**：滑动窗口（例如 `hls_list_size 8`）并删除窗外分段；关停时取消 janitor。

### 4.14 PIN 失败计数在锁外自增

`getPINAttempt` 释放锁后，`attempt.failures++` 无保护。同一 IP 并发猜会打穿「5 次锁 15 分钟」。局域网还豁免了 token-bucket，锁就是 LAN 上唯一节流。

---

## 5. P1：架构债（现在不大，库一大就疼）

### 5.1 媒体索引是两套存储

`internal/cache` 把整份 `MediaResponse` JSON 放内存再原子写 `*.media_cache.json`；同时 `media/store.go` 又把同一批条目写入 SQLite。读路径：内存 → DB → 全量扫描。

结果：大库时 JSON 内存峰值和落盘 I/O 双倍；缓存失效语义两套；`MediaProcessor` 既管 ffmpeg 又管入库。

**建议**：SQLite 当唯一索引源；内存只留 ETag + 分页查询。JSON 磁盘缓存可删。

### 5.2 两个「上帝包」

| 包 | 实际职责 |
|----|----------|
| `internal/handler` | HTTP + 限流 + IP/PIN + HLS + 转码策略 + 缩略图 + 日志上报 |
| `internal/media` | probe / 转码 / HLS / 硬件加速 / **DB 索引** |

`service` 层偏薄。CodeMap 写的 Clean Architecture 比代码更干净。`media/store.go` 直接拿 `*gorm.DB`；`SQLite.DB()` 把 GORM 漏给 media 包。没有 `MediaStore` 接口。

**建议**：不要一次大拆。先把 `store.go` 挪到 `storage` 或 `service/index`；限流/IP 抽到 `internal/httpx`。handler 只做解析和状态码。

### 5.3 死代码在膨胀覆盖率

- `internal/types`：纯 re-export，全仓库零引用。
- `constants.StatusOK` 等：handler 全部用 `net/http`，只被 `constants_test.go` 测「200 == 200」。
- `domain.AppError` 未使用。
- `cmd/msp/main_test.go` 是空壳。

Codecov 徽章会好看，对回归几乎没帮助。测试预算应留给流式超时、HLS 清理、配置热重载、缩略图 allowlist。

### 5.4 Schema 只有 `AutoMigrate`

索引 `Exec` 的错误被丢掉。`AutoMigrate` 不能做破坏性变更、不能回滚。进度 ID 迁移写在 `main.go` 里，是 delete-then-insert，崩溃会丢那一行。

**建议**：`schema_migrations` 表 + 按版本跑 SQL；索引创建要查错。

### 5.5 本地 preset 名不副实

armv7 的条件是 platform `arm` 而不是 `linux`：`-P linux` 没有 armv7，`-P arm` 只有 armv7、没有 arm64。Windows 的 `-L` 是跳过 lint，Unix 的 `-L` 是列出 preset。`build.sh` 无参数时默认 windows/x64，即使在 Linux 上。

---

## 6. P2：前端工程化

`web/package.json` 只有 `dev` / `build` / `preview`。仓库里有 `eslint-disable` 注释，但没有 ESLint。`web/web_test.go` 只测 embed FS。

| 项 | 现状 | 建议 |
|----|------|------|
| Lint / format | 无 | `oxlint` 或 ESLint + Prettier，CI 加 `bun run lint` |
| 类型 | JSDoc `@typedef` | `// @ts-check` 或逐步 `.d.ts` |
| 测试 | 仅 embed 编译 | 先测纯函数：`sort-filter`、`i18n`、`diff.js`、PIN、playlist scope |
| 第三方库 | `public/assets/` 手拷 plyr / hls.js / pinyin-pro | 改 npm 依赖，让 Dependabot 真正更新 |
| 版本 | `"version": "0.0.0"` | 跟 Go release 对齐或删掉该字段 |
| SW 缓存 key | hash `package.json` | 应 hash `web/bun.lock` |
| CSP | `script-src 'unsafe-inline'` + `cdn.plyr.io` | 资源已本地化，去掉多余源和 inline |

其它前端问题：

- 全局可变 `state`，任意模块可改 `tab` / `playlist`。
- `applyConfigToUI` 每次 `config:loaded` 把 `state.tab` 重置为 `ui.defaultTab`，和 304 分支的 tab 修复打架。
- PIN 对话框：无 `inputmode="numeric"`、无 maxlength、无焦点陷阱；429 `locked` 当普通失败。
- `setMeta` 用 `innerHTML`；Open Raw 字幕用字符串拼 HTML。
- 搜索 `/pattern/flags` 在主线程 `new RegExp`，ReDoS 会冻 UI。
- 侧栏 tab 没有 `role="tab"`；搜索框只有 placeholder。
- DB 不可用提示写死中文，未走 i18n。

播放器状态是可变全局单例，对这个体量可以接受。更值钱的是给 `sort-filter` / `diff` / `api` 降级路径加测试。

---

## 7. P2：安全加固

这些在纯家庭 LAN 上不是紧急漏洞，但代码已经按 Tunnel/Remote 分支了。

1. **`CF-Connecting-IP` 在 RemoteAddr 为回环时无条件信任。** 文档写不信任代理头，`trustProxy` 未使用。本机任意进程都能带这个头改写白名单。`CF-Ray` 会把访问标成 Remote。应仅在显式 `behindCloudflare` 时启用。
2. **`GET /api/config` 免 PIN。** LAN 未认证即可看到分享标签、IP 黑白名单、播放/转码配置。PIN 开启时 GET 只返回 `{ pinEnabled, accessLevel }` 即可引导 UI。
3. **限流对整个 LAN 豁免。** PIN 锁在内存里，重启即清。
4. **`.svg` 被当作可 inline 媒体。** 恶意 SVG 在 `image/svg+xml` 下可执行脚本。播放路径是 `img.src = streamUrl`。应 `Content-Disposition: attachment` 或移出图片页。
5. **Service Worker 把已认证缩略图当公共资源缓存**（`StaleWhileRevalidate` + 后端 `Cache-Control: public`）。PIN 过期后同一浏览器仍可能出库里的图。
6. **Session 纯内存、7 天、不轮转、无登出。** 重启全员掉线；被偷的 cookie 7 天有效。应在 SECURITY 里写清楚：PIN 是家庭共享锁，不是多用户 IAM。
7. **HSTS 信任 `X-Forwarded-Proto`**，与「不信任代理」矛盾，可在明文局域网名上注入 HSTS。
8. **HLS 播放列表是能力 URL。** ID 不可猜，但日志会打源路径；泄露 m3u8 在空闲超时前可用。
9. **共享 PIN = 共享进度/收藏/偏好。** 一人一个家没问题，不要把它当多用户认证。
10. **golangci 只开了 8 个 linter。** 建议加 `govulncheck`、`errorlint`、`bodyclose`、`gocritic`。Actions 钉的是 major（`@v4`），不是 SHA。
11. **Gzip 在 handler 跑之前就设 `Content-Encoding: gzip`**，304 / 空响应可能坏掉。
12. **默认监听所有网卡 + PIN 关闭。** 对「插上网线就能播」没问题，但已接 Cloudflare Tunnel 语义。启动时若检测到非回环且 PIN 关闭，应打醒目警告。

---

## 8. 构建 / CI / 发布（补充）

已经做得好的：check.yml 先编前端再 `go test -race`、vet、lint、全架构矩阵；`bun.lock` + `--frozen-lockfile`；Dependabot 覆盖 Go / npm / Actions。

缺口：

| 项 | 说明 |
|----|------|
| 前端在矩阵里编 8 次 | 应单独 frontend job，把 `web/dist` 当 artifact |
| Bun cache key | hash `package.json` 而不是 `bun.lock` |
| `push` + `pull_request` | 同仓 PR 会跑两遍 |
| Codecov `fail_ci_if_error: false` | 覆盖率是装饰 |
| golangci 无 `formatters` | CONTRIBUTING 写「强制 gofmt」，CI 没强制 |
| 无 Windows/macOS runner | race 只在 Linux |
| 本地 `bun install` 无 `--frozen-lockfile` | 依赖会漂 |
| checksum 大小写 | PowerShell `Get-FileHash` 大写，CI `sha256sum` 小写 |
| Dependabot 无 `docker` | 基础镜像从不升级 |
| `go install github.com/blycr/msp@latest` | 模块名是 `msp`，这条路径永远不可用（已文档化） |
| README 无 Docker 一节 | 用户只能看 compose |
| `/healthz` 在 `db: false` 时仍 HTTP 200 | 作 readiness 没用，要看 JSON |
| `.gitignore` 漏了 | `data/`、`media/`、`thumbs/`；`docker compose up` 后容易把片源提交进去 |

---

## 9. 可观测性与运维

- 日志已收到 `slog`，但没有 request id，转码失败和前端 `/api/log` 对不上。
- 没有转码队列深度、扫描耗时、缓存命中、ffmpeg 进程数。`/healthz` 只有 `status/db/uptime`。家庭场景加一个本机 only 的 `GET /api/status` 即可。
- `CheckFFmpeg` 每次 `GET /api/config` 都打「FFmpeg found」（默认 warning 所以平时安静）。
- 配置热重载仍是 2 秒 `Stat` 轮询。Windows 下编辑器原子替换有时摸不到 mtime，文档应写「改完若没生效等 2 秒 / 重启」。
- API 错误形状不统一：`{"error":{"message"}}` / `{"error":"db_unavailable"}` / 嵌入 `Error` / 405 无 body。中英混用。调用方查 `error.message` 会漏掉 DB 降级。

---

## 10. 文档与仓库卫生

- `docs/API_REFERENCE.md` 的 `GET /api/config` 示例仍返回完整 `port/shares`，没写 access-level 过滤。
- `docs/archive/` 里多份旧评审（明文 PIN、无 CI）会误导后来者。建议在 archive 首页加一行「截至 v1.12 已修复」。
- `CHANGELOG.md` 无日期、与 `docs/release/vX.Y.Z.md` 双份维护，容易只改一边。
- PR 模板没有强制「前端改了要说明如何手工验证」——而前端正好没有自动测试。
- `dev.ps1` 有热键和 FileSystemWatcher；`dev.sh` 是 mtime 轮询，还依赖 `jq`。

---

## 11. 与历史评审对照

| 旧结论 | 来源 | 现在 |
|--------|------|------|
| AES-GCM 随机 nonce / MediaID 不稳 | sonnet | 已修：HMAC 派生 nonce + 进度迁移 |
| `globalIDKey` 包级全局 | verification | 已修：`IDCodec` 注入 |
| PIN 明文存储/比较 | kimi | 已修：bcrypt `pinHash` |
| 无 PR/push CI | kimi | 已修：`check.yml` |
| RateLimiter map 无上限 | kimi / deepwiki | 部分修：上限 + 随机驱逐，无 TTL |
| `pinAttempts` 从不驱逐 | deepwiki | 部分修：上限 1000 |
| CF-Connecting-IP / CF-Ray 可伪造 | deepwiki | **仍有效** |
| EncodeID 静默 base64 降级 | deepwiki | **仍有效** |
| 配置热重载 2s 轮询 | kimi | 仍如此，非安全缺陷 |
| Firefox `audioMeta` | README | 仍为已知问题 |
| 无前端测试 | — | **仍有效** |
| 缩略图缺 `IsAllowedFile` | — | **新发现** |
| 设置 POST 清 PIN | — | **新发现** |
| SVG inline XSS | — | **新发现** |
| SW 公共缓存缩略图 | — | **新发现** |
| folder playlist 仍当 ID 是路径 | — | **新发现**（加密 ID 后的回归） |
| SECURITY.md 仍写旧 PIN 模型 | — | **新发现**（文档漂移） |
| release 文件名重复 suffix | — | **新发现** |
| tag 不跑 check.yml | — | **新发现** |

---

## 12. 请求路径（备忘）

```
HTTP :8099
  WithRecovery → WithLog → WithSecurity → WithRateLimit → WithAdminLockdown → WithGzip
    → ServeMux
         /api/*     → handler.Handler
         /healthz   → inline JSON
         /          → embedded web.FS
```

典型媒体请求：handler 解 opaque `id` → 再对当前 shares 做路径校验 → 列表走 cache → processor → scanner + SQLite；转码/HLS/probe/缩略图直接打到 `*media.MediaProcessor`。

分层现状：HTTP 边缘基本是六边形，核心是一个偏胖的 media 对象。没有循环依赖。当前形状（胖 handler + 胖 MediaProcessor + 薄 service）匹配产品体量，不必重写成「干净架构」。P1 是具体缺陷，不是缺抽象。
