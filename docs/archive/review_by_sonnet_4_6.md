好，我把我评审中指出的每个问题，都写成可以直接发给 AI 的独立 Prompt。按严重程度排序，每条 Prompt 都是**自包含的**——告诉你需要粘贴什么代码，AI 读完就能直接动手。

---

# Prompt 1【最重要】根治 AES-GCM 随机 nonce 导致 MediaID 不稳定

> 这是 v1.5.1 里 `OnConflict(path)` 补丁背后的真正根因。

```
## 背景与问题

我的项目 MSP 是一个 Go 局域网媒体服务器。

v1.5.0 引入了 AES-GCM 加密来生成媒体 ID，但使用了随机 nonce，
导致每次扫描同一文件都会产生不同的 ID。
v1.5.1 的临时补丁是把 SQLite 的 UpsertMediaItems OnConflict 列
从 id 改为 path，这只是救火，根因未解决：
  - 书签/历史记录中的旧 ID 永久失效
  - DB 里会积累大量孤儿记录
  - 每次扫描都做不必要的 UPDATE

## 根治方案

用确定性 ID 替代随机 nonce：对文件路径 + 服务端固定密钥做
HMAC-SHA256，取前 16 字节做 base64url 编码。
相同路径 + 相同密钥 = 永远相同 ID，且路径不可逆推。

## 需要你修改的文件

请阅读以下代码，然后完成修改任务：

### 当前 ID 生成代码
[把 internal/media/ 或其他包里生成 mediaID 的函数粘贴到这里]

### 当前密钥加载代码
[把 msp.key 的读取/生成逻辑粘贴到这里]

### 当前 sqlite.go UpsertMediaItems
[粘贴 internal/storage/sqlite.go 中 UpsertMediaItems 函数]

### 当前 store.go 扫描入库逻辑
[粘贴 internal/media/store.go 中 IndexMediaToDB 函数]

## 修改要求

1. 新建或修改 ID 生成函数，签名为：
   func DeriveMediaID(filePath string, serverKey []byte) string
   实现：HMAC-SHA256(key=serverKey, data=filePath)，
   取摘要前 16 字节，base64.RawURLEncoding 编码返回。

2. 在服务启动时加载 msp.key（现有逻辑），把 key 注入到
   MediaProcessor 或对应结构体中，供 DeriveMediaID 使用。
   不允许使用 package-level 全局变量存储 key。

3. UpsertMediaItems 的 OnConflict 列改回 id（因为 ID 现在是确定性的，
   相同文件永远产生相同 ID，不再需要用 path 救火）。

4. IndexMediaToDB 中每次生成 mediaID 时调用 DeriveMediaID，
   不再调用旧的随机 nonce 函数。

5. 为 DeriveMediaID 编写测试：
   - 相同输入返回相同输出（幂等性）
   - 不同路径返回不同输出
   - 空路径不崩溃
   - 输出只包含 base64url 安全字符

6. 删除旧的随机 nonce 生成函数（如果它只服务于 mediaID 生成）。

## 输出格式

每个修改的文件用 ```go 文件路径 包裹，输出完整文件内容（不是 diff）。
最后输出一段迁移说明：现有用户升级后旧 ID 全部失效，
应如何在 README/CHANGELOG 中告知用户清空数据库重新扫描。
```

---

# Prompt 2 修正 go.mod 模块名并全库替换 import 路径

```
## 问题

我的 Go 项目 go.mod 第一行是：
  module msp

这违反了 Go 模块命名规范。应该是：
  module github.com/blycr/msp

模块名错误会导致：外部无法正确 go get 该模块；
IDE 的跳转和自动补全在某些情况下异常。

## 需要你做的事

### 第一步：确认影响范围

请分析以下目录结构，列出所有需要修改 import 路径的 .go 文件：
[粘贴 find . -name "*.go" | head -50 的输出，或目录树]

### 第二步：生成修改方案

输出一个可直接执行的 shell 脚本，完成以下操作：

1. 修改 go.mod 第一行为 module github.com/blycr/msp
2. 用 sed 将所有 .go 文件中的：
     "msp/internal/  →  "github.com/blycr/msp/internal/
     "msp/cmd/       →  "github.com/blycr/msp/cmd/
   （注意：只替换 import 块中的路径，不要误改注释或字符串字面量）
3. 运行 go build ./... 验证编译通过
4. 运行 go vet ./... 验证无警告

脚本要求：
- 使用 bash，兼容 macOS 和 Linux
- 每步操作前打印正在执行什么
- 任何步骤失败立即 exit 1

### 第三步：同步更新文档

找出以下文件中所有引用模块路径的地方并给出修改后内容：
[粘贴 README.md 和 CONTRIBUTING.md 的内容]

## 输出格式

1. fix_module_name.sh（完整脚本）
2. 受影响文件的完整列表
3. README.md / CONTRIBUTING.md 的修改 diff
```

---

# Prompt 3 清理文档中 Gin 的错误引用

```
## 问题

项目 README.md 的 Acknowledgements 部分写着：
  - Gin - HTTP web framework written in Go.

但当前 go.mod 的直接依赖中没有 Gin：
[粘贴你的 go.mod 内容]

这说明项目已经迁移离开了 Gin（可能改用标准库 net/http + chi，
或者纯 net/http），但文档没有同步更新，会误导贡献者。

## 需要你做的事

### 第一步：确认当前 HTTP 层实现

请阅读以下文件，判断项目当前使用的 HTTP 路由方案：
[粘贴 cmd/msp/main.go]
[粘贴 internal/handler/ 目录下任意一个 handler 文件，如 media.go]
[粘贴 internal/server/ 或 internal/http/ 下的路由注册文件]

### 第二步：修改 README.md

根据第一步的结论：

情况 A：如果完全使用标准库 net/http，则：
- 删除 Gin 的 Acknowledgements 条目
- 在 Highlights 或 Build from Source 下加一句：
  "Zero framework dependency: HTTP server built on Go standard library."

情况 B：如果使用了 chi 或其他路由库，则：
- 将 Gin 替换为实际使用的库，更新链接和描述

情况 C：如果代码里还有 Gin 的 import 但 go.mod 没有，
说明有编译错误，请指出具体文件和行号。

### 第三步：检查其他文档

检查 docs/ 目录下是否有其他文档提到 Gin：
[粘贴 docs/ 目录的文件列表]

## 输出

1. 确认结论（使用的是什么 HTTP 方案）
2. 修改后的 README.md Acknowledgements 部分完整内容
3. 需要同步修改的其他文档列表及修改内容
```

---

# Prompt 4 根治 Firefox audioMeta 黑块

```
## 问题

README 中有这样一条已知问题：
  "Firefox users: The audio metadata panel (audioMeta) may occasionally
   render as a black block."
当前的"解决方案"是建议用户换用 Chrome，这不是修复。

## 需要你做的事

请阅读以下代码，定位并修复根本原因：

### 相关 CSS
[粘贴 web/src/ 中包含 audioMeta 相关样式的 CSS 文件，
 通常在 components/audio.css 或类似路径]

### 相关 JS
[粘贴 web/src/modules/ 中负责渲染音频元数据面板的 JS 文件]

### 已知背景

Firefox 的 audioMeta 黑块通常由以下几个原因之一导致：
A. 使用了 backdrop-filter，Firefox 对其支持不完整
B. 使用了 mix-blend-mode，Firefox 在某些合成层情况下渲染异常
C. 元素有 background: transparent 但父元素有 will-change 或
   transform，Firefox 的合成层隔离导致背景变黑
D. 使用了 CSS 渐变 + opacity 动画，Firefox 的位图缓存导致黑块

## 修复要求

1. 定位具体原因（A/B/C/D 或其他），并在回复开头说明是哪种情况。

2. 提供 CSS 修复方案，要求：
   - 不改变视觉效果（其他浏览器下外观不变）
   - 使用 @supports 或 @-moz-document 做 Firefox 专项处理（
     如必要），不影响 Chrome/Safari 的渲染质量
   - 如果 backdrop-filter 是根因，提供不依赖该属性的备选实现

3. 提供 JS 端的防御性代码（如适用）：
   - 如果是动态挂载/卸载 DOM 导致的问题，在挂载后强制触发
     一次重绘：element.style.display='none'; element.offsetHeight;
     element.style.display=''

4. 修复后，从 README 中删除那条 Firefox 已知问题注释。

## 输出

1. 根因诊断
2. 修改后的完整 CSS 文件（```css 文件路径 包裹）
3. 修改后的 JS 文件（如有改动）
4. 修改后的 README.md 相关段落
5. 手动验证步骤（如何在 Firefox 中确认修复有效）
```

---

# Prompt 5 补充测试覆盖率 Badge 和 CI 覆盖率上报

```
## 问题

项目没有测试覆盖率徽章，也没有在 CI 中上报覆盖率，
导致覆盖率水平对贡献者不透明。

## 需要你做的事

请阅读当前 CI 配置：
[粘贴 .github/workflows/ 下的所有 yml 文件内容]

请阅读当前 README.md 的徽章部分：
[粘贴 README.md 开头的徽章行]

### 第一步：修改 CI workflow

在现有的 test job 中，将测试命令从：
  go test ./...
改为：
  go test -race -coverprofile=coverage.out -covermode=atomic ./...
  go tool cover -func=coverage.out

并添加上报步骤（使用 Codecov，免费且支持公开仓库）：
  - name: Upload coverage
    uses: codecov/codecov-action@v4
    with:
      files: ./coverage.out
      fail_ci_if_error: false

要求：
- 保留原有的 -race 检测（如果有的话，如果没有请加上）
- coverage 步骤失败不应阻断整个 CI（fail_ci_if_error: false）
- 在 workflow 的 permissions 块加 contents: read

### 第二步：在 README.md 添加覆盖率徽章

在现有徽章行追加：
[![codecov](https://codecov.io/gh/blycr/msp/branch/main/graph/badge.svg)](https://codecov.io/gh/blycr/msp)

### 第三步：添加本地覆盖率查看命令到 Makefile 或 scripts/

如果项目有 Makefile，添加：
  coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    open coverage.html  # macOS；Linux 用 xdg-open

如果没有 Makefile，在 scripts/README.md 中加入对应的
bash one-liner 说明。

## 输出

1. 修改后的完整 CI workflow yml 文件
2. 修改后的 README.md 徽章行
3. Makefile 新增的 coverage target（或 scripts 说明）
4. 需要在 codecov.io 完成的手动步骤（授权 GitHub App 等）
```

---

# Prompt 6 补充集成测试（最高价值的缺失测试）

```
## 背景

项目目前有较好的单元测试，但缺少集成测试。
最高价值的集成测试是验证"扫描 → 入库 → API 返回"完整链路。

## 需要你做的事

请阅读以下文件：
[粘贴 internal/app/media_service.go 或等效的 Service 文件]
[粘贴 internal/storage/sqlite.go]
[粘贴 internal/handler/media.go]
[粘贴 internal/server/ 或路由注册文件]

编写集成测试文件 internal/handler/integration_test.go

### 测试环境要求
- 使用 SQLite in-memory 数据库（不创建文件）
- 使用 testdata/ 目录下的真实测试媒体文件（如没有，
  用 os.CreateTemp 创建最小的合法 mp4/mp3 占位文件）
- 用 httptest.NewServer 启动真实 HTTP 服务（不 mock Handler）
- 每个测试用例独立数据库，用 TestMain 或 t.Cleanup 清理

### 必须覆盖的场景（按优先级）

1. TestScanThenList：
   - 添加一个包含 3 个 .mp4 文件的临时目录为共享目录
   - 触发扫描（POST /api/shares 或对应端点）
   - 等待扫描完成（轮询 GET /api/media 直到 scanning:false）
   - 验证 GET /api/media 返回 3 个视频项

2. TestMediaIDStability：
   - 对同一文件扫描两次
   - 验证两次返回的 MediaID 完全相同（deterministic ID 的核心保证）

3. TestStreamRange：
   - 对一个已入库的媒体文件发送 Range: bytes=0-1023 请求
   - 验证返回 206 Partial Content
   - 验证 Content-Range header 格式正确
   - 验证返回体长度为 1024 字节

4. TestPINBruteForce：
   - 连续发送 5 次错误 PIN（POST /api/pin）
   - 第 6 次验证返回 429 Too Many Requests
   - 验证 Retry-After header 存在

5. TestLANAccessControl：
   - 模拟来自非本机 IP 的请求（设置 RemoteAddr 为 192.168.1.100:12345）
   - 验证 GET /api/config 不返回敏感字段（如 pin 值）

### 代码规范

- 用 testify/assert 和 testify/require
- 每个测试函数包含注释说明测试意图
- 辅助函数提取到 testhelper_test.go（同包）
- 构建 tag：//go:build integration
  使普通 go test ./... 不运行，
  需要 go test -tags integration ./... 才运行

## 输出

1. internal/handler/integration_test.go（完整）
2. internal/handler/testhelper_test.go（辅助函数）
3. testdata/ 目录结构说明和生成脚本（如需要）
4. 在 .github/workflows/ 中添加 integration-test job 的 yml 片段
5. 在 scripts/README.md 或 Makefile 中添加运行命令
```

---

## 使用建议

按这个顺序发，前面的修复会影响后面的测试：

```
Prompt 1 → Prompt 2 → Prompt 3 → Prompt 4 → Prompt 5 → Prompt 6
（AES-GCM）  （模块名）  （文档）   （Firefox）  （覆盖率）  （集成测试）
```

每条 Prompt 发完后，先把 AI 输出的代码在本地编译验证（`go build ./...` + `go test ./...`），确认没有回归，再发下一条。不要一次性把 6 条全发给同一个对话——上下文太长会导致 AI 在后期产生前后不一致的输出。