# 归档评审

本目录存放历史评审与方向规划文档。**它们记录的是撰写当时的状态，其中的缺陷多数已经修复。**

阅读前请注意：

| 结论 | 现状（截至 v1.12.2） |
|------|---------------------|
| 明文 PIN / PIN 可被绕过 | 已修：bcrypt `pinHash`，失败计数锁内更新 |
| AES-GCM 随机 nonce、MediaID 不稳定 | 已修：HMAC-SHA256 派生确定性 nonce，并提供进度 ID 迁移 |
| 包级全局 `globalIDKey` | 已修：`util.IDCodec` 依赖注入 |
| 无 PR/push CI | 已修：`.github/workflows/check.yml`，含 race 测试、lint、8 架构编译矩阵 |
| RateLimiter map 无上限 | 部分修：有上限与随机驱逐，仍无 TTL |
| 配置文件无校验 | 已修：`config.Validate` 接入加载 / API / 热重载 |
| 设置页保存会清空 PIN | 已修：写回时保留已有 `pinHash` |
| 缩略图未校验分享目录 | 已修：与 `/api/stream` 共用 `resolveMediaTarget` |
| SVG 可内联执行脚本 | 已修：改为 `Content-Disposition: attachment` |
| Service Worker 把缩略图当公共资源缓存 | 已修：`Cache-Control: private`，SW 去掉 `StaleWhileRevalidate` |
| 前端零测试 | **仍成立**（无前端测试运行器） |
| `EncodeID` 静默降级 base64 | **仍成立**（`msp.key` 创建失败已改为启动失败，但 `IDCodec` 的 nil-key 分支仍在） |
| `CF-Connecting-IP` / `CF-Ray` 可伪造 | **仍成立**（有意不信任代理头，文档已说明） |

当前状态与待办以 [docs/ENGINEERING_REVIEW.md](../ENGINEERING_REVIEW.md) 为准；该文件的"核实与落地"一节逐条记录了哪些建议已落地、哪些是误判、哪些有意不改。

历史文档：

- `review_by_*.md`、`AI_REVIEW_VERIFICATION*.md` —— 外部模型评审与逐条核实
- `plan_direction*.md` —— 早期方向规划（多设备、媒体库、音频）
- `evolution_analysis.md` —— 项目演进分析
