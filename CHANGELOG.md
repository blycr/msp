# Changelog

## 1.9.2

- **全屏/大尺寸自适应修复**：
  - 音频页歌词区不再固定 320px：audioMeta 改弹性布局填满舞台剩余高度，歌词区随之撑满，全屏（F11）或大屏下不再出现舞台下方大片空白；封面相对歌词列垂直居中
  - 视频/图片真正铺满舞台：`#audioEl` 同步 `hidden` 属性（此前用 inline display 隐藏，导致 `.playerbox:has(#audioEl:not([hidden]))` 恒匹配、playerbox 常驻 20px padding），现在非音频模式 padding 归零、媒体零缝隙贴边
  - 三栏布局侧栏限宽：`minmax(260px, 0.82fr)` 改为 `minmax(260px, 360px)`，超宽屏下侧栏不再随视口线性拉宽，多余宽度让给舞台

## 1.9.1

- **修复翻页按钮在全屏/窗口变高后悬空**：
  - 根因：pager 只是追加在条目后的普通子元素，没有机制钉在底部；播放列表的自动测页只在渲染后触发，未监听尺寸变化（`plAutoFit.ro` 声明了却从未接线）
  - 列表容器改 flex 列布局，`.pager` 以 `margin-top: auto` 兜底贴底
  - 播放列表接上 ResizeObserver，尺寸变化（F11、拖拽窗口）时自动重新测页
  - 文件列表新增同款自动测页（取代固定每页 10 条），页大小随面板高度自适应，保留当前页首项位置不跳动
  - 文件夹视图为全量滚动无 pager，不涉及

## 1.9.0

- **前台沉浸式去框化（UI 重设计）**：
  - 设计规则统一：一个区域一个面，面内不用边框，分隔靠留白与色调层次；边框只保留给真正的浮层（dialog、dropdown 菜单、plyr 菜单/工具提示）
  - 三栏面板去除边框，靠背景与面板的柔和色差分区；tabs/search/playlist/hint/pager/breadcrumb/footer 等处内部分隔线全部移除
  - 控件去描边：ghost 按钮、图标按钮、输入框、开关、标签页、badge 均改为无框填充式，hover/active 只变化底色
  - 舞台（播放区）随主题适配：浅色主题与面板同色，暗色主题深于面板；新增 `--stage-*` 设计 token（bg/text/sub/hover/active），舞台内标题、按钮、开关、歌词、封面、空态、转码提示、plyr 音频控件全部 token 化并随主题切换
  - 视频铺满舞台：拆除 playerbox 的边框/阴影/外边距，修正 plyr 包装层撑满舞台，letterbox 区域与舞台同色
  - 设置对话框精简：section 去底色改为纯留白分组（修复输入框与 section 底色相同导致输入框不可见），移除标题/操作区分隔线与 `<hr>` 分隔符
  - 移动端同步：mobile-nav 去边框/阴影，playerbox 贴边
  - 清理无用的 `--stage-media-bg`/`--shadow-soft` token 及与之冲突的暗主题专项覆盖
- **修复歌词高亮位置**：
  - 当前歌词行由偏上（35%）改为精确垂直居中（50%）
  - 修复移动端看不到高亮歌词：根因是 `offsetTop` 相对最近定位祖先（`.playerbox`）而非歌词容器计算，滚动量被高估，桌面端把高亮行顶到遮罩淡化区、移动端直接滚出可视区；改用 `getBoundingClientRect` 做布局无关计算

## 1.8.0

- **前端 UI 设计系统统一**：
  - 统一 SVG 图标系统：`icons.js` 重构为图标注册表，单一出口 `icon(name)`；Settings 改为滑杆图标、Refresh 改为 refresh-cw；清除全部文本字形图标（📁/★/☆/←/▶ 全部 SVG 化），项目内不再出现 emoji 图标
  - 新增通用分页组件 `ui/pager.js`：文件列表与播放列表共用（chevron 图标按钮），删除 3 处重复实现；播放列表自动测页逻辑不变
  - 全局隐藏滚动条（滚动功能不变）；新增设计 token（`--ring`/`--stage-bg`/`--stage-media-bg`/`--shadow-dialog`），焦点环、播放器沉浸底、弹窗阴影全部 token 化，暗色主题自动适配
  - 按钮体系统一：`.icon-btn`/`.theme-btn`/`.fav-btn` 同一规格；按钮阴影改 `color-mix` 随主题变化
  - 响应式媒体工作区改进：移动端导航、面板布局与组件样式整体打磨
  - 修复 `.theme-btn svg { fill: currentColor }` 覆盖图标本体导致日/月图标被意外填充；修复空态 play 图标 data-URI 被 CSP 拦截（改为真实 SVG 元素）
- **后端性能优化**：
  - 日志改为异步缓冲写入：`logChan`（容量 4096）+ 单一写 goroutine 落盘，写满丢弃不阻塞请求处理
  - 多共享目录并行扫描：每目录一个 goroutine + WaitGroup 聚合，大媒体库首扫/重扫更快
  - `/api/media` 新增 ETag 快速路径：`If-None-Match` 命中缓存 ETag 时直接 304，不再构建响应
  - SQLite 连接池从固定 1 调整为按 GOMAXPROCS；新增表达式索引（`LOWER(name)`、扫描排序复合索引、progress/favorites 时间索引）
  - GC 调参：`SetGCPercent` 50 → 100
- **构建与工程**：
  - `build.ps1`/`dev.ps1`：Windows PowerShell 5.1 下检测到 pwsh 自动以其重新执行（修复部分 5.1 环境缺失 `Get-FileHash` 导致全量编译失败）
  - Dockerfile：后端构建镜像升级 `golang:1.25-alpine`，与 go.mod 对齐
  - 修复 auth/thumbnail handler 的 golangci-lint 警告
  - 文档同步：CodeMap（接口/流程/版本号）、scripts/README（pwsh 依赖说明）


## 1.7.4

- **随机播放算法改进（洗牌包 / shuffle bag）**：
  - 不再使用固定顺序复用，改为一轮内每个文件恰好播一次、绝不重复（Fisher-Yates 全排列）
  - 一轮播完后自动重新洗牌生成新随机序列，循环模式下每轮顺序不同
  - 跨轮边界避免上一轮末尾立即成为新一轮首位（堵掉"刚听过又来"的体验缺陷）
  - 关闭 shuffle 时立即恢复顺序播放，游标定位到当前项不打断
- **修复 shuffle 状态泄漏**：
  - `state.playlist.shuffle` 从前是全局态，所有类型（video/image）的 `buildPlaylist` 都读它，但 UI 开关只对 audio 暴露→video/image 被悄悄随机化又无法切换
  - 修复：`buildPlaylist` 仅 `kind === "audio"` 时才读 shuffle，从源头消除泄漏
- **修复左侧标签页切换后的翻页脱节 bug**：
  - 后台 `resumeLast()` 和 304 缓存命中分支会强制覆盖 `state.tab` 为用户上次播放类型（audio），且不更新 `.tab--active` 视觉→点翻页时左侧列表变成音频，但标签页条仍高亮视频
  - 修复：`renderList()` 自动同步 `.tab--active`；`resumeLast` 加模块守卫仅首次启动恢复 tab；删除 304 分支直接覆盖 `state.tab` 的逻辑
  - 切换标签页时重置 `state.listPage = 1`，避免跨类型页码残留
- 变更文件：`web/src/modules/playlist/navigation.js`、`web/src/modules/ui/render.js`、`web/src/modules/ui/bindings.js`、`web/src/modules/player/resume.js`、`web/src/modules/actions.js`

## 1.7.3

- **视频缩略图稳定性全面修复**：
  - 修复并发首次加载时因非阻塞信号量（2 槽）导致的大量 429 拒绝，改为**阻塞排队 + 8s 超时**（4 槽），突发批量请求不再被丢弃
  - 排队后二次检查缓存，避免重复运行 ffmpeg
  - 新增短视频回退机制：`-ss 5` 失败时自动回退到首帧（`-ss 0`），消除 <5s 视频的缩略图 404
  - 错误响应添加 `Cache-Control: no-store`，确保前端重试能真正重新请求，不会被浏览器缓存阻挡
  - 删除 ffmpeg 失败后的空/损坏缓存文件，避免后续"缓存存在但无效"分支
- **前端缩略图重试与降级**：
  - 新增 `setupThumbRetry()`：`onerror` 触发指数退避重试（3 次：400/800/1600ms），应对临时拥塞或生成延迟
  - 彻底失败时干净隐藏 `<img>`（`.file-thumb--failed { display: none }`），不再显示碎图图标
  - 仅修改 3 个文件：`internal/handler/thumbnail.go`、`web/src/modules/ui/render.js`、`web/src/styles/components/list.css`

## 1.7.2

- **Dockerfile**：修复 Go 版本号错误（`1.25` → `1.24`），`docker build` 不再失败；固定 Alpine 镜像版本；移除无用的 `gcc`/`musl-dev`；容器以非 root 用户运行
- **构建脚本**：修复 `scripts/build.sh` 中 4 处 bash `&&`/`||` 运算符优先级 bug
- **日志系统**：修复 `RotateLogIfNeeded()` 与 `Log()` 之间的数据竞争；缓存 `log.Logger` 实例消除每次调用的堆分配

## 1.7.1

- **构建修复**：修复 `golangci-lint` 中 `gosec` 扫描器的误报问题，为可信路径调用和文件存在性检查添加 `//nolint:gosec` 注释
- **符号链接处理**：`transcoder.go` 使用 `os.Lstat` 替代 `os.Stat`，正确拒绝符号链接输入
- **CI**：移除发布工作流中冗余的测试任务

## 1.7.0

- **恢复播放行为重新设计**
  - 移除"继续观看"（Continue Watching）列表及侧边栏区域
  - 移除 `GET /api/progress/recent` 端点和 `RecentProgress` 领域类型
  - 点击媒体项仍自动恢复上次播放进度，核心行为不变

- **日志系统重新设计**
  - 新增 `Warning` 日志级别，支持 Info / Warning / Error 三级分类
  - 日志同时输出到文件和控制台（dual output），便于开发调试
  - `internal/service/logger.go` 重构，支持级别过滤与多输出目标

- **设置对话框视觉层级优化**
  - 引入分组标题与分隔线，设置项按功能聚类（播放/网络/安全/外观）
  - 优化表单控件对齐与间距，统一 Neo-Industrial 设计规范

- **其他改进**
  - 修复音视频进度条与音量条的鼠标指针样式及拖拽文字选中问题
  - 测试：`TestMediaServiceGetOrBuildMediaCache` 消除后台任务竞态
  - 工程：禁用 Git `autocrlf`，消除 Windows 环境 LF/CRLF 警告

## 1.6.3

- **继续观看（Continue Watching）**
  - 后端：新增 `RecentProgress` 领域类型；`ProgressStore` / `Store` 新增 `ListRecentProgress(limit int)` 方法，按 `updated_at` 降序分页查询。
  - 后端：新增 `GET /api/progress/recent?limit=N` 端点，默认上限 10 条。
  - 前端：启动时自动拉取最近 5 条进度并与媒体库 join 映射，渲染为侧边栏顶部续播卡片。
  - 前端：搜索模式下自动隐藏"继续观看"区域，避免与搜索结果竞争视觉焦点。
  - 前端：点击续播条目直接调用 `playItem()` 并恢复进度，无需二次确认。

- **文件夹层级浏览（Folder Browse）**
  - 后端：`MediaItem` 新增 `RelPath` 字段；`scanner.go` 在扫描阶段通过 `filepath.Rel` 计算相对于 share root 的目录路径。
  - 前端：新建 `folder.js`，从扁平 `mediaItems` 实时提取目录树结构，支持无限层级嵌套。
  - 前端：`render.js` 支持 `flat` / `folder` 双模式渲染；文件夹模式显示面包屑导航、子文件夹列表和当前目录文件。
  - 前端：侧边栏新增 Folder / Flat 模式切换按钮；状态扩展 `browseMode`、`currentFolder`。

- **收藏/标记（Favorites）**
  - 后端：新增 `Favorite` 领域类型和 `FavoriteStore` 接口（List/Add/Remove/Is）。
  - 后端：`SQLite` / `Store` 实现收藏 CRUD；`AutoMigrate` 自动创建 `favorites` 表。
  - 后端：新增 `/api/favorites` REST 端点（GET / POST / DELETE），添加操作幂等。
  - 前端：启动时批量拉取收藏状态到 `favoriteIds` Set，本地即时更新并后台同步。
  - 前端：文件列表项右侧渲染星标按钮（☆/★），点击即时切换并同步后端。
  - 前端：新增第五个 `Favorites` Tab，跨视频/音频/图片类型聚合显示已收藏媒体，支持搜索、排序、分页。
  - 前端：收藏 Tab 下自动隐藏"继续观看"和文件夹浏览控件，保持界面聚焦。

- **视频缩略图（Video Thumbnails）**
  - 后端：新增 `GET /api/thumbnail?id=xxx` 端点，调用 ffmpeg 截取第 5 秒画面输出 JPEG。
  - 后端：缩略图缓存到 `<exe_dir>/thumbs/<media_id>.jpg`，带 `If-Modified-Since` 和 24h HTTP 缓存头。
  - 后端：信号量限制最多 2 个并发 ffmpeg 缩略图生成任务，避免进程风暴；ffmpeg 不可用时返回 404。
  - 前端：视频列表项左侧渲染缩略图，使用 `loading="lazy"` 延迟加载，ffmpeg 不可用时优雅降级为默认图标。

- **转码进度反馈（Transcode Status）**
  - 前端：播放视频/音频时，若 probe 判定需要转码，播放器上方显示浮动状态条。
  - 前端：状态机分"检测兼容性..."和"正在转码，请稍候..."两阶段，后者带旋转 CSS 动画。
  - 前端：媒体元素触发 `canplay` 后状态条自动淡出消失，消除"网络慢"与"正在转码"的不确定性。

## 1.6.2

- **代码质量**：`internal/handler/auth.go`：`sync.RWMutex` → `sync.Mutex`（仅使用 `Lock()`，无读锁场景）。
- **效率**：`cmd/msp/main.go`：启动 PIN 迁移增加守卫条件，无明文 PIN 时跳过无意义的 `UpdateConfig` 调用，减少启动 I/O。
- **日志**：`cmd/msp/main.go`：统一启动日志格式 `Warning:`（与项目其余日志风格一致）。
- **代码质量**：`internal/server/server.go`：`checkAndReloadConfig` 预计算 `needsSave`，扁平化深层嵌套条件。
- **工程改进**：`internal/server/server.go`：`saveConfigLocked` 保存成功后自动 `os.Stat` 并同步 `cfgModTime`，消除 `checkAndReloadConfig` 调用方的重复同步逻辑，同时修复 `UpdateConfig` 不更新 `cfgModTime` 导致后续不必要重载的问题。
- **代码清理**：撤销 `internal/handler/common.go` 引入的 `evictRandomEntry` 泛化函数，恢复 `auth.go` 和 `middleware.go` 的内联驱逐逻辑（更直接可读，避免为抽象而抽象）。
- **文档归档**：两份 AI 评审核验报告已归档至 `docs/archive/`（`AI_REVIEW_VERIFICATION.md`、`AI_REVIEW_VERIFICATION_KIMI.md`）。

## 1.6.1

- **安全：PIN 明文存储 → bcrypt 哈希**
  - `SecurityConfig` 新增 `PINHash` 字段，config.json 中不再持久化明文 PIN。
  - 新增 `config.SanitizeSecurity`：自动将明文 PIN 哈希为 bcrypt 并存入 `PINHash`，清空 `PIN`。
  - `auth.go`：`HandlePIN` 验证改为 `bcrypt.CompareHashAndPassword`。
  - 启动时自动迁移旧版明文 PIN；配置热重载后新 PIN 也会自动哈希。
- **安全：RateLimiter / PIN 暴力破解防护的内存上限**
  - `RateLimiter` 新增 `maxSize = 10000`，超限后随机驱逐，防止 IP 轮换攻击导致的内存无限增长。
  - `pinAttempts` 新增 `maxPinAttempts = 1000`，超限后随机驱逐。
- **工程：消除 sqlite.go DRY 违反**
  - 提取 `guard()` / `guardTx()` 内部 wrapper，替换 14 处重复的 `if s.db == nil` 守卫逻辑。
- **可观测性**
  - `DeleteByShareRootsNotIn` 在执行无条件全表删除时输出 `[WARN]` 日志。

## 1.6.0

- **修复：确定性 MediaID（根治 v1.5.1 临时补丁）**
  - `internal/util/crypto_id.go`：`EncodeID` 的 nonce 从随机 `io.ReadFull(rand.Reader)` 改为 `HMAC-SHA256(key, path)[:nonceSize]` 确定性派生。同一文件始终生成相同 ID。
  - 新增 `IDCodec` 结构体，彻底消除 `globalIDKey` 包级全局变量，改为依赖注入。
  - `internal/storage/sqlite.go`：`UpsertMediaItems` 的 `OnConflict` 从 `path` 改回 `id`，恢复标准主键冲突策略。
  - `cmd/msp/main.go`：启动时自动执行 `PlaybackProgress` 旧 `media_id` 迁移（旧 ID → DecodeID 还原路径 → 新确定性 ID → 更新记录），失败仅记录日志不中断启动。
- **架构：全面依赖注入化**
  - `internal/media/processor.go`：`MediaProcessor` 持有 `idCodec`。
  - `internal/scanner/scanner.go` / `subtitle.go`：`WalkShares`、`FindSidecarSubtitlesCached` 等函数接收 `idCodec` 参数。
  - `internal/handler/handler.go` / `stream.go`：`Handler` 通过 `Deps.IDCodec` 注入，`stream.go` 使用 `h.idCodec.DecodeID`。
- **文档修复**
  - `README.md`：删除 Acknowledgements 中已移除的 Gin 框架引用。
  - `README.md`：更新 Firefox 已知问题说明（已做 GPU 层兼容性处理）。
- **CI / 覆盖率**
  - `.github/workflows/check.yml`：`go test` 增加 `-coverprofile=coverage.out`。
  - `.github/workflows/check.yml`：新增 coverage artifact 上传步骤。
  - `README.md`：增加 Codecov coverage badge。
- **可观测性**
  - `internal/storage/sqlite.go`：12 个数据库操作函数在 `s.db == nil` 时增加 `[WARN]` 日志，消除静默失败。
- **测试**
  - 新增 `internal/handler/integration_test.go`：覆盖扫描→入库→API 返回全链路、MediaID 稳定性、Range 请求、字幕集成。
- **配置验证**
  - `internal/config/validate.go`：`isValidIP` 改为 `net.ParseIP`，支持 IPv6；`isValidIPOrCIDR` 改为 `net.ParseCIDR`，支持 IPv6 CIDR。
- **代码清理**
  - `internal/cache/media.go`：`LoadFromDisk` 消除冗余布尔逻辑（`already || !need` 恒等于 `already`）。

## 1.5.1

- **修复：添加共享目录后前端不显示媒体**
  - `internal/media/store.go`：`LoadMediaResponseFromDBScan` 添加 `copy(resp.Shares, shares)`；`IndexMediaToDB` 先清理旧数据再扫描，避免 DB 唯一约束冲突；batch 内增加路径去重。
  - `internal/storage/sqlite.go`：`UpsertMediaItems` 的 `OnConflict` 从 `id` 改为 `path`（适配 AES-GCM 随机 nonce 导致每次扫描 id 不同的问题）。
  - `web/src/modules/ui/render.js`：`renderList()` 改为检查实际媒体项（videos/audios/images/others）而非仅 shares 数组。
  - `web/src/modules/actions.js`：`loadMedia(true)` 后若 `scanning: true` 自动启动轮询（1.5s / 15 次），无需手动刷新。
- **修复：控制台双重时间戳日志**
  - `internal/service/logger.go`：移除手动时间戳拼接，消除 `log.Println` 与 `log.SetFlags` 的双重时间戳。
- **修复：Local/LAN 访问触发 429 Too Many Requests**
  - `internal/handler/media.go`：`refreshCooldown` 仅对非 Local 生效。
  - `internal/handler/middleware.go`：LAN 访问豁免限流；限流范围收紧为仅 4 个管理端点（`POST /api/pin`、`GET /api/media?refresh=1`、`POST /api/config`、`POST /api/shares`），stream/probe/progress/subtitle/log 等正常播放 API 不限流。
- **修复：播放时 `TypeError: Cannot read properties of null (reading 'play')`**
  - `web/src/modules/player/play.js` 和 `core.js`：所有 `state.plyr.play()` 调用前添加 null 检查，防止 Plyr `ready` 异步回调执行时实例已被销毁。

## 1.5.0

- **安全审计全面修复**（14 项漏洞，P0–P3）：
  - **PIN 暴力破解防护**：5 次错误后锁定 15 分钟，per-IP 计数器
  - **取消弱默认 PIN**：`DefaultPIN` 从 `"0000"` 改为 `""`
  - **常数时间 PIN 比较**：`crypto/subtle.ConstantTimeCompare`
  - **配置死锁防护**：`pinEnabled=true` + `pin=""` 自动降级为 `false`
  - **全局限流器**：Token-Bucket（per-IP，Local 豁免），`/api/pin` 1/5s、`/api/media?refresh=1` 1/30s
  - **三级访问分级**：Local（完整功能）/ LAN（隐藏设置+过滤配置）/ Remote（清空敏感字段+管理 API 403）
  - **Cloudflare Tunnel 识别**：回环+CF 头时识别为 Remote
  - **TOCTOU 竞态修复**：Open 后二次 `EvalSymlinks` + `IsAllowedFile`
  - **Inline XSS 防护**：非媒体文件强制 `Content-Disposition: attachment`
  - **Refresh DoS 冷却**：全局 30s
  - **CSP + HSTS**：完整 Content-Security-Policy；HTTPS 时 HSTS
  - **AES-GCM 媒体 ID 加密**：替代 base64 路径编码，启动时自动生成 `msp.key`
  - **客户端日志注入防护**：level 白名单 + 消息截断 500 字符 + 换行过滤
  - **WriteTimeout 60s**、IP 黑白名单 CF 修复、移除废弃 `X-XSS-Protection`
- **前端访问分级配套**：设置按钮按环境显示/隐藏、分级提示翻译
- **⚠️ 破坏性变更**：旧 base64 ID 书签失效（需重新收藏）

## 1.4.0

- **前端样式重构与组件化**：
  - 清理 `app.css` 中 5 组重复定义和 2 处死代码（`h1-h6`、`textarea:focus-visible`），移除冗余 `!important` 12 个
  - 将 1,385 行的 `app.css` 按组件拆分为 10 个文件（`base.css`、`layout.css`、`components/*`、`vendor/plyr.css`、`responsive.css`）
  - 原生 `<select>` 排序下拉菜单替换为自定义 Dropdown 组件，完全适配 Neo-Industrial 设计系统（硬阴影、主题色、圆角、dark/light 模式）
  - 自定义 Dropdown 支持键盘导航（↑↓ Enter Esc）和 ARIA 无障碍属性
- **翻译修复**：补全 `share_settings` 缺失的 i18n 翻译（英文 "Folders" / 中文 "共享目录"），消除中文模式下显示英文 key 的问题
- **构建脚本并行化**：`build.sh` 和 `build.ps1` 支持本地并发交叉编译（GNU parallel / Start-Job），多目标构建速度提升约 66%

## 1.3.0

- **MediaProcessor 架构重构**：将 `internal/media/` 包中所有包级全局变量收敛到 `MediaProcessor` 结构体
  - 新增 `processor.go`：统一持有 `mediaDB`、`probeCache`、`transcodeLimit`、`hwAccel` 等全部原先分散在 4 个文件中的全局状态
  - `NewMediaProcessor(db, opts...)` 构造函数采用 Option 模式（`WithTranscodeLimit`）
  - `cacheTTL` 改为 `atomic.Int64`，`hwDisabled` 改为 `atomic.Bool`，消除数据竞争
  - 每个实例拥有独立的 `sync.Once`，支持并行测试互不干扰
  - 迁移函数：`store.go`/`probe.go`/`transcoder.go`/`hwaccel.go` 的核心函数全部改为 `MediaProcessor` 方法
  - 删除全局函数：`SetDB`、`SetTranscodeLimit`、`ResetPathsForTest`、`ResetHWAccelForTest`
  - `media.go` 提取 `newMediaResponse` 公共构造函数，消除重复构造逻辑
- **调用方适配**：`server.New`、`cache.NewMediaCache`、`handler.New`、`service.NewConfigService` 签名更新，显式注入 `MediaProcessor`
- **测试适配**：所有 media 包测试改用 `NewMediaProcessor`；为常用方法添加 nil-receiver 安全保护；修复 `stream_test.go` pre-existing `gosec G306` 警告

## 1.2.2

- **前端内存泄漏修复**：
  - `destroyPlyr()` 现在对称清理 `window.plyrPlayer`、`window.callPlyr`、`stallCheckTimer`、`volumechange`/`ratechange` 监听器
  - `audio-track.js` 的 `setInterval` 和事件监听器支持通过 `cleanupAudioTrackHandling()` 清理
  - `play.js` 切换媒体前 `revokeObjectURL` 释放旧的 Blob URL
  - `api.js` 对非 JSON 响应（如 502 HTML）不再静默返回 `{}`，改为抛出包含状态码和原文的错误
  - `lyrics.js` 缓存歌词节点数组，避免每次 `timeupdate` 重复 `querySelectorAll`
  - 搜索框增加 200ms debounce，减少快速输入时的连续重渲染
  - `resume.js` 全局热键增加 `unbindGlobalHotkeys()` 对称清理接口
- **后端错误处理与资源管理**：
  - `cache/media.go`：`buildAndUpdate` 返回 `error`，媒体构建/序列化失败时跳过缓存更新，避免空数据被缓存
  - `service/logger.go`：日志轮转 reopen 失败时回退到 `os.Stderr`，防止后续日志全部静默丢失
  - `handler/config.go`：用 `errors.Is` + sentinel errors 替代脆弱的字符串匹配判断错误类型
  - `handler/middleware.go`：`getClientIP` 改用 `net.SplitHostPort`，正确支持 IPv6 地址；`statusWriter` 增加 `Unwrap()` 支持 Go 1.20+ 的 `http.ResponseController`
- **测试补充**：新增 `common_test.go`（`writeJSON`、`decodeJSONBody`、`isPayloadTooLarge`）和 `stream_test.go`（`decidePlaybackMode`、`checkTranscodePolicy`、`resolveMediaTarget`）；修复 `subtitle_test.go` 已有的编译错误

## 1.2.1

- **测试覆盖率提升**：
  - 新增 `internal/handler/auth_test.go` 和 `internal/service/session_test.go`，补全认证流程与会话管理逻辑的测试。
  - 重构 `stream_test.go`、`subtitle_test.go`、`server_test.go` 和 `util_test.go`，补全边界条件测试。
- **代码质量优化**：
  - 修复 `internal/handler/stream_test.go` 中 `unusedparams` 诊断警告，移除未使用的 `configPath` 参数。
  - 修复测试文件中未检查的错误返回值，统一符合 Go 最佳实践。

## 1.2.0

- **FFmpeg 多路径发现**：FFmpeg/ffprobe 查找从单一 PATH 升级为 7 层优先级搜索（环境变量 → 同目录 → bin/ → CWD → 平台路径 → PATH），支持 `MSP_FFMPEG_PATH` 环境变量指定路径
- **启动日志修正**：`FormatHWAccelStatus` 区分 3 种状态（unavailable/software/hardware），FFmpeg 不可用时转码并发上限自动归零
- **后端播放策略**：`/api/probe` 新增 `playback.mode` 字段，后端基于实际编码信息（byte-sniff + ffprobe）决策 direct/transcode，覆盖 H.264/H.265/AV1/VC-1/AC-3/DTS/TrueHD 等主流编码
- **前端播放集成**：新增 `getPlaybackUrl()` 异步函数查询后端策略，替代前端扩展名猜测逻辑，消除 5-10 秒回退延迟
- **CSS 性能优化**：移除不可见的 `backdrop-filter`，topbar 模糊半径 12px→8px 并启用 GPU 合成层，歌词滤镜改为透明度
- **代码清理**：移除 `probeWarnText()`、`needsCompatibilityVideoTranscode()`、AVI 特殊分支等死代码
- **测试补强**：新增 FFmpeg 路径发现、播放策略决策等 31+ 测试用例

## 1.1.4

- **依赖安全修复**：升级 vite、vite-plugin-pwa、workbox-window，修复 serialize-javascript RCE、fast-uri 路径遍历、@babel 任意代码执行等漏洞
- **文档整理**：4 个一次性分析文档移入归档，更新 API 参考和配置示例文档

## 1.1.3

- **Bug 修复**：修复 ffprobe 缓存键精度丢失，`string(rune(mtime))` 改为 `fmt.Sprintf`，消除不同文件误共享缓存的问题
- **性能优化**：
  - ffprobe 编码探测从两次进程调用合并为单次（`-of json`），扫描速度提升约一倍
  - Scanner 正则黑名单规则改为预编译缓存（`sync.Map`），避免每次匹配重复编译
  - 前端 probeCache 容量从 100 提升至 500，减少大型媒体库浏览时的重复请求
  - 前端播放进度上报改为 300ms 窗口批量合并
- **CI 优化**：重构 CI 工作流，引入 matrix 构建和 artifact 传递

## 1.1.1

- **前端架构深度重构**：
  - `player.js` 从 579 行精简至 51 行，转为薄编排层（仅 re-export + bus 监听）
  - `playlist.js` 从 175 行精简至 45 行，同样转为薄编排层
  - 提取 `player/play.js`（`playItem` + `onMediaEnded`），消除 `player.js ↔ player/resume.js` 循环依赖
  - 将 `setPlaylist` 移入 `playlist/navigation.js`，通过 `bus.emit('playlist:updated')` 触发 UI 更新
  - 消除 `actions.js ↔ ui/bindings.js` 循环依赖，改用 bus 事件解耦
- **代码去重**：
  - `rememberEnabled` 从 3 处实现统一为 `api.js` 中的单一实现
  - `canStorage` 从 2 处实现统一为 `state.js` 中的单一实现
- **死代码清理**：移除 7 个未使用的导出（`I18N`、`mediaErrorText`、`getSortVal`、`hidePinDialog`、`verifyPin`、`switchAudioTrack`、`getAudioTracks`）和 1 个无监听者的 bus 事件
- **PWA 优化**：VitePWA 启用 `cleanupOutdatedCaches`、`skipWaiting`、`clientsClaim`，加速 Service Worker 更新
- **I18N 完善**：歌词纯音乐提示文案改为 i18n key，页面标题和 `lang` 属性统一为英文
- **构建脚本**：新增龙架构（loong64）Linux 构建支持

## 1.1.0

- **统一错误处理**：
  - 消除静默忽略错误，统一 HTTP 错误响应格式为 JSON
  - 新增 panic 恢复中间件 `WithRecovery`，防止服务端崩溃
  - 重构 `writeJSON` 为 buffer 模式，编码失败返回 500 JSON 而非空响应
- **单元测试覆盖率大幅提升**：
  - 新增约 1800 行测试代码，新增 10 个测试文件
  - `cache`、`media`、`service`、`handler` 等核心包覆盖率从 0% 提升至已覆盖
- **构建脚本全面重构**：
  - 引入预设系统，新增 `scripts/build-profiles.json`，内置 10 个预设
  - 参数简化：单字母别名（`-P`、`-F`、`-A`、`-T`、`-L` 等）
  - 中文兼容性修复：脚本全部改为英文，避免 PowerShell 5.1 解析失败
- **文档体系完善**：
  - 新增 `scripts/README.md` 和 `scripts/README_CN.md` 构建脚本文档

## 1.0.0

- **正式版发布**：MSP 迎来第一个正式版本，达到生产就绪状态
- **后端架构升级**：
  - 提取领域类型到 `internal/domain/`，零外部依赖
  - 新建 `internal/storage/` 包，实现接口化存储
  - 从 `media` 包拆出 `internal/scanner/`，职责更清晰
  - 提取 `internal/cache/media.go`，支持内存 + 磁盘缓存
  - 新增业务编排 Service 层，Handler 层依赖接口而非具体实现
- **前端架构升级**：
  - 新增 EventBus 发布/订阅事件总线，解耦模块间通信
  - Player 模块拆分为 6 个文件，Playlist 模块拆分为 4 个文件
- **新特性**：
  - 优雅关机：监听 `SIGINT`/`SIGTERM`，自动终止 FFmpeg 转码进程
  - ffprobe 结果缓存：使用 `sync.Map` + TTL（5 分钟）
  - 正则预编译：提升 scanner 包性能
- **脚本和工作流改进**：构建脚本新增参数、开发脚本支持优雅关闭、CI 并发控制
- **文档整理**：迁移 `README_CN.md` 到 `docs/` 目录，归档历史文档

## 0.9.1

- **音频转码默认开启**：`playback.audio.transcode` 默认值由 `false` 改为 `true`，避免 Firefox 等浏览器遇到不支持的音频格式时报错
- **前端格式兼容性增强**：
  - 扩展 `mimeFor()` 支持的格式映射（视频新增 `.flv`、`.ts`、`.m2ts` 等；音频新增 `.wma`、`.ape`、`.mka` 等）
  - 放宽 `canPlayMedia()` 判断逻辑，未知格式允许尝试直接播放
- **Firefox 兼容性**：针对 Firefox 浏览器 `audioMeta` 区域偶发黑块问题添加兼容性处理
- **依赖更新**：`vite` 7.3.1 → 8.0.10，Go 依赖升级至最新版本

## 0.9.0

- **新特性：硬件加速转码**：
  - 新增 FFmpeg 硬件加速编码支持，自动探测可用硬件编码器
  - 支持 `h264_nvenc`（NVIDIA）、`h264_qsv`（Intel）、`h264_amf`（AMD）、`h264_vaapi`（Linux）、`h264_videotoolbox`（macOS）
  - 配置项 `playback.video.encoding` 新增 `hwAccel` 和 `maxJobs` 字段
  - 硬件编码失败自动标记禁用，无感回退到软件编码

## 0.8.12

- **播放策略收敛（低侵入）**：
  - 预转码触发条件收敛为 `AVI/WMV` 容器，避免 `MP4` 等兼容格式误触发预转码
  - 保持"直连优先，失败回退转码"主链路不变
- **播放器稳定性修复**：
  - 增强媒体错误处理兜底，避免错误处理器内部异常导致页面二次崩溃
  - 转码回退切源增加播放器实例守卫，必要时自动降级到原生媒体元素
  - 修复智能拖动路径中的变量作用域问题
- **WMV 兼容补强**：
  - 前端 MIME 映射补全：`.wmv -> video/x-ms-wmv`
- **文档更新**：
  - 新增 `docs/release/v0.8.12.md`
  - 同步更新 `docs/API_REFERENCE.md` 与 `docs/CONFIG_EXAMPLE.md` 的播放策略描述

## 0.8.11

- **播放器兼容策略精修（家庭场景）**：
  - 引入"探测感知 + 直连优先 + 单次回退"策略，继续保持非侵入式行为
  - 对 `AVI/WMV` 以及探测到高风险编码（如 HEVC/VC-1/AC-3/DTS/TrueHD）的视频，优先预转码
  - 保持兼容格式（如常见 H.264/AAC 的 MP4）优先直连，不默认转码
  - 直连失败后保留一次带时间戳刷新（`ts`）的重试，再执行自动转码回退，降低误判
- **稳定性修复**：
  - 修复播放器错误回调中变量遮蔽导致的 `TypeError: c is not a function`
  - 增强媒体切换阶段的空源错误保护，避免瞬时错误被误判为永久失败
- **格式支持**：
  - 补全 `.wmv` 媒体分类与 `Content-Type` 映射（`video/x-ms-wmv`）
- **文档**：
  - 新增发布说明：`docs/release/v0.8.11.md`
  - 同步更新 Wiki 编码兼容策略说明与变更记录

## 0.8.10

- **安全与稳健性优化**：
  - 关键 JSON 接口统一增加请求体大小限制（1 MiB），超限返回 `413`
  - 修复媒体 `limit` 标记语义，仅在真实截断时设置 `Limited=true`
  - 字幕转换接口增加大文件保护（8 MiB），避免异常文件导致内存风险
  - 家庭局域网模式下移除 HTTPS 推断：PIN 会话 Cookie 固定非 `Secure`，客户端 IP 固定取 `RemoteAddr`
- **测试补强**：
  - 新增 `internal/handler/handlers_safety_test.go` 覆盖上述安全边界
- **文档**：
  - 详细说明见 `docs/release/v0.8.10.md`

## 0.8.9

- **代码质量修复**：
  - 修复 test 文件中未检查的错误返回值（`resp.Body.Close()`、`os.RemoveAll()` 等）
  - 修复文件操作权限过于宽松的问题（`os.MkdirAll` 权限 0755 → 0750，`os.WriteFile` 权限 0644 → 0600）
  - 简化 `ClassifyExt` 函数，移除复杂的长度分支逻辑，降低圈复杂度
  - 应用 De Morgan 定律简化条件表达式，删除未使用的 `sessionTimer` 字段
- **技术改进**：所有 golangci-lint 检测问题已清零

## 0.8.8

- **随机播放逻辑重构**：
  - 修复随机模式下播放列表每次点击都重新打乱的问题
  - 新增 `playOrder` 数组存储播放顺序，保持原始列表有序
  - 切换随机模式时保持当前歌曲位置，仅重排后续顺序
  - 修复上一首/下一首在随机模式下的导航逻辑
  - 修复播放列表高亮显示与随机顺序不同步的问题
- **文档清理**：移除全仓库 emoji，保持简洁专业风格

## 0.8.7

- **代码质量重构**：
  - 提取魔法数字为常量，集中管理端口、权限、缓存时间等配置
  - 统一错误消息为常量，便于维护和多语言支持
  - 为 40+ 个导出函数添加中文文档注释
  - 消除重复代码，提取 `buildMediaCacheAndUpdate` 公共函数
  - 改进错误处理，添加更多上下文信息（`fmt.Errorf` 包装）
- **性能优化**：
  - 优化 `ClassifyExt` 函数，减少 `strings.ToLower` 调用，提升媒体扫描性能
  - 优化 `buildMediaItem` 中的字符串处理
- **测试覆盖**：
  - 新增 `constants` 包完整测试
  - `util` 包测试覆盖率从 20.9% 提升至 65.5%
- **代码清理**：
  - 删除未使用的 Windows 别名函数（`NormalizeWinPath`, `WithinWinRoot`, `SamePathWin`）

## 0.8.6

- **代码质量重构**：媒体扫描/入库/响应构建与部分 API Handler 逻辑拆分，降低复杂度、便于维护。
- **行为不变**：以重构为主，无破坏性改动。

## 0.8.5

- **UI 优雅升级**：视频播放器与图片预览全面圆角化；修复列表项高度塌陷；优化暗黑模式下输入框与按钮质感。
- **播放器增强**：修复 Tooltip 被容器遮挡问题（重构 CSS Overflow 逻辑）；修复快速切换视频时的 `canPlayType` 报错。

## 0.8.4

- **播放策略重构**：实施 "Try-First, Fallback-Next" 策略。不再根据文件后缀判断，而是优先尝试原生播放。如果浏览器拒绝（如 AVI）或解码失败（如 H.265），前端自动请求转码。
- **AVI/MKV 修复**：完美修复 AVI 等格式在 Chrome 中无法播放的问题。通过 `canPlayType` 智能预判，对不支持的格式直接启用转码，避免下载弹窗。
- **稳定性增强**：
  - 优化播放器看门狗（Watchdog），防止转码启动慢导致的误杀。
  - 修复播放列表切换时的转码状态丢失问题。
  - 恢复并优化 Smart Seeking，转码流现在支持拖动进度条。
- **后端优化**：实现转码并发限制（默认 2 路），防止 CPU 耗尽。
- **配置更新**：`config.example.json` 默认开启转码以支持智能回退。

## 0.8.3

- **移动端体验全面重构**：
  - 单列布局：重新设计移动端界面，采用垂直流（播放器 → 播放列表 → 文件列表）
  - 整个应用页面在移动端可滚动，解决小屏幕内容被截断问题
  - 音频播放器高度自适应，封面尺寸优化（160px），歌词视图完全可见
- **Bug 修复**：修复音频播放提前 1-2 秒停止的问题（新增自动恢复机制）

## 0.8.2

- **前端视觉升级（Neo-Industrial）**：
  - 采用新设计语言，使用 Bricolage Grotesque 和 Instrument Sans 字体
  - 全局隐藏滚动条设计，实现更简洁的类应用体验
  - 重新设计"共享设置"对话框，统一按钮、输入框高度（36px）和圆角
- **后端优化**：优化启动顺序，先初始化日志再连接数据库，抑制 "SLOW SQL" 等噪音

## 0.8.1

- **数据库健壮性优化**：
  - 修复媒体扫描过程中由于路径索引不一致导致的 `UNIQUE constraint failed: media_items.id` 错误
  - 将 `UpsertMediaItem` 的冲突检测目标从路径迁移至主键，确保媒体入库的绝对可靠性

## 0.8.0

- **数据库优化**：新增 `playback_progresses` 表，将高频播放进度更新从 `user_prefs` 表中分离，显著提升性能并减少数据库膨胀
- **智能转码**：新增 FFmpeg 服务端转码支持，自动识别并转码浏览器不支持的格式（如 MKV, FLAC, AVI）
- **API 新增**：新增 `/api/progress` 端点，专门用于处理播放进度
- **文档**：更新 wiki 增加转码配置说明；明确转码模式下暂不支持拖拽进度条（Seeking）

## 0.7.0

- **安全增强**：新增基于 IP 的黑/白名单机制，支持单个 IP 和 CIDR 范围（/8, /16, /24）
- **PIN 认证**：新增 PIN 码认证功能，默认 PIN 为 0000，可在配置文件中自定义
- **配置扩展**：在 `config.json` 中新增 `security` 配置部分
- **API 端点**：新增 `/api/pin` 端点用于 PIN 验证
- **文档完善**：新增安全配置指南（`docs/SECURITY.md`）和配置示例文档（`docs/CONFIG_EXAMPLE.md`）
- **测试覆盖**：为安全功能添加完整的单元测试

## 0.6.0

- **UI/UX 全面升级**：遵循 UI/UX Pro Max 设计规范，替换 emoji 为 SVG 图标，优化颜色对比度和交互体验
- **交互改进**：所有可交互元素添加 hover 反馈和键盘导航支持，统一过渡动画为 200ms
- **无障碍性**：添加 `aria-label`、支持 `prefers-reduced-motion`、改进焦点可见性
- **响应式优化**：改进移动端布局和对话框显示
- **构建修复**：修复 PowerShell 构建脚本中 npm 调用问题

## 0.5.8

- **重构**：后端全面引入 `context.Context`，支持请求超时与优雅取消
- **修复**：修复 Gosec SQL 拼接误报与部分编译错误
- **安全**：数据库查询全面升级为参数化与 Context 感知

## 0.5.7

- **重构**：核心模块拆分为 Scanner/Store/Media 三层架构
- **CI**：集成 GitHub Actions 进行自动构建与 Lint 检查
- **质量**：引入 golangci-lint 规范代码风格

## 0.5.6

- 特性：服务启动后自动打开浏览器并彩色输出访问地址
- 改进：CI 发布流程加固与加速（Go 缓存、依赖优化）

## 0.5.5

- 新增 `scripts/dev.ps1`，支持后端自动重编译 + 前端联动开发
- 开发模式默认关闭"自动打开浏览器预览"，正式构建不受影响
- 配置与黑名单字段在缺省时自动补齐，兼容旧配置
- Vite 开发服务器支持局域网访问，便于手机端调试
- 列表与播放列表分页条布局优化（Prev/Next 左右、页码居中）
- Footer 文案与结构重排，统一品牌展示与多语言文案
- `.gitignore` 补充本地运行产物/构建输出忽略项

## 0.5.3

- **首屏更快（分段加载）**：
  - `GET /api/media` 支持 `limit` 参数，返回值包含各分类总数以及 `limited=true`
  - 支持 `ETag/If-None-Match`，命中返回 `304 Not Modified`
  - 服务端媒体列表缓存采用 stale-while-revalidate 策略
  - 运行时缓存落盘到 `config.json.media_cache.json`
- **探测更丰富**：`GET /api/probe?id=...` 返回容器、音视频编码信息及外挂字幕列表
- **PWA 优化**：Service Worker 对 `/api/*` 使用 NetworkOnly，避免 API 被离线缓存

## 0.5.2

- **Go 原生跨平台构建**：
  - 新增脚本参数支持按需选择平台与架构，默认仅生成 Windows x64
  - 产物统一生成校验（SHA256）与调试拷贝目录
- **CI 工作流**：默认生成全部平台与架构，并将产物与校验文件打到 Release
- **路径跨平台规范化**：后端统一使用 NormalizePath / WithinRoot / SamePath
- **前端平台提示优化**：共享目录对话框的路径占位符根据操作系统自动切换

## 0.5.1

- **性能优化**：
  - 左侧文件列表与播放列表支持分页（默认每页 10 项）
  - 音频播放器与图片预览采用与视频一致的轻量淡入过渡
- **体验优化**：明暗主题切换过渡统一为轻量淡入，尊重系统"减少动态效果"设置
- **文档补充**：PWA 使用方法、从源码构建步骤、隐私提交流程

## 0.5.0

- **后端重构**：将后端从单文件实现拆分为更清晰的内部模块（config / handler / media / server / util / web / types）
- **前端工程化升级**：引入 Vite 构建，前端目录结构调整为 `web/src` 与 `web/public`
- **构建与分发**：增加 Windows 构建脚本（PowerShell），调整嵌入式静态资源组织方式
- **PWA 支持**：增加 PWA 相关资源（manifest 与 Service Worker）

## 0.4.1

- **CI / Release 工作流增强**：支持手动触发发布流程（workflow_dispatch），支持自定义版本号与发布说明

## 0.4.0

- **多语言（I18N）**：前端新增中英双语界面（EN / ZH），常用按钮与提示支持语言切换
- **搜索与排序体验**：文件名搜索增强（拼音 / 正则 / 模糊匹配），新增排序选项
- **UI 与播放体验**：优化整体布局与动画表现，修复自动播放等体验问题
- **文档**：增加英文 README，并对文档结构与入口做了调整

## 0.2.0

- 修复：PC 端 Plyr 全屏视频四周黑边（支持 cover/contain 切换）
- 修复：`/api/stream` 为常见媒体类型返回更稳定的 `Content-Type`
- 改进：前端静态资源统一收敛到 `web/static`，embed 逻辑更清晰
- 改进：`config.json` 作为本机运行时配置不再提交；提供 `config.example.json`
- 改进：非视频页面隐藏"填充模式"按钮，交互更一致

## 0.1.0

- 初版：局域网媒体共享与浏览器预览播放
