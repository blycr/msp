MSP 项目综合评价（基于代码审查）
对标范围：同类型家庭/个人局域网媒体服务器（filebrowser、go-file、miniserve 等同体量项目）

一、各维度代码审查
1. 架构与依赖注入
handler.go 的 Deps 结构体和接口定义（ConfigProvider、MediaCacheProvider、SessionProvider、Logger）设计清晰，依赖注入完整。 handler.go:16-46

但存在明显矛盾：Deps.Processor 字段直接暴露了 *media.MediaProcessor 具体类型，而非接口，违反了依赖倒置原则。同一文件中其他依赖都是接口，唯独这个是具体类型。 handler.go:44

auth.go 中 pinAttempts、pinAttemptsMu 是包级全局变量，media.go 中 lastRefreshTime、refreshMu 也是包级全局变量，scanner.go 中 regexpCache 同样是包级全局变量。这与项目声称的"消除全局变量"目标相矛盾，说明依赖注入改造并不彻底。 auth.go:20-25 media.go:13-17 scanner.go:26-27

2. 安全实现
正面：resolveMediaTarget 中的 TOCTOU 防护（Open 后二次 EvalSymlinks + IsAllowedFile）实现正确，是同体量项目中少见的安全细节。 stream.go:85-97

constantTimeCompare 实现正确，通过填充到相同长度再比较，防止长度侧信道泄漏。 auth.go:123-133

问题1：getAccessLevelFromRequest 中的 Cloudflare Tunnel 识别逻辑存在安全漏洞——CF-Connecting-IP 和 CF-Ray 头可以被任何客户端伪造。代码只检查"回环IP + CF头"，但没有验证这些头的来源合法性，攻击者可以从本机发送带伪造 CF 头的请求来绕过 Remote 限制。 middleware.go:434-441

问题2：pinAttempts map 永不清理，长期运行下每个访问过的 IP 都会在内存中留下记录，构成内存泄漏。 auth.go:20-37

问题3：EncodeID 在 AES/GCM 初始化失败时静默回退到 plain base64，调用方无法感知安全降级。 crypto_id.go:50-53

3. 并发与内存安全
MediaProcessor 中 atomic.Int64（probeTTL）和 atomic.Bool（hwAccel.disabled）使用正确，sync.Once 保证 FFmpeg 路径只探测一次。 processor.go:28-43

问题1：probe.go 中 ClearProbeCache 直接赋值 mp.probeCache = sync.Map{}，sync.Map 是值类型，这个赋值操作本身不是原子的，在并发场景下存在数据竞争。 probe.go:34-36

问题2：middleware.go 中 isPrivateIPv4 每次调用都执行三次 net.ParseCIDR，这些 CIDR 是固定常量，应该预编译为包级变量。 middleware.go:414-419

问题3：RateLimiter.buckets map 永不清理，长期运行下大量不同 IP 的 bucket 会持续积累，构成内存泄漏。 middleware.go:181-191

问题4：cache/media.go 中每次构建完成后调用 go debug.FreeOSMemory()，这会强制 Go runtime 将内存归还给 OS，但会触发 STW GC，对于媒体服务器这类需要低延迟的场景是不合适的。 media.go:208

4. 数据库操作
SQLite 配置合理：WAL 模式、synchronous=NORMAL、PrepareStmt: true、单连接限制，是 SQLite 嵌入式使用的标准最佳实践。 sqlite.go:55-67

问题：DeleteByShareRootsNotIn 当 shareRoots 为空时执行全表删除（AllowGlobalUpdate: true），这是高危操作。虽然调用方 cleanupStaleData 在传入空列表时理论上不应该调用，但代码层面缺乏额外保护。 sqlite.go:279-281

sqlite.go 中大量重复的 if s.db == nil { log.Printf(...); return nil } 防御性检查，代码重复度高，可以用一个辅助方法统一处理。 sqlite.go:91-96

5. 媒体处理
decidePlaybackMode 的编解码器判断逻辑完整，覆盖了 H.264/H.265/AV1/VC1/AC-3/DTS/TrueHD 等主流格式，"有画无声"问题有专门处理。 stream.go:283-332

SniffContainerCodecs 通过读取文件头尾字节嗅探编解码器，但 sniffByExt 只支持 .mkv、.mp4、.m4v、.mov，.avi、.webm、.wmv 等格式直接返回空字符串，覆盖不全。 scanner.go:318-326

硬件加速探测有平台过滤（VAAPI 仅 Linux，VideoToolbox 仅 macOS，AMF 仅 Windows），设计合理。 hwaccel.go:81-99

6. 前端代码
eventbus.js 22 行实现完整，错误隔离（try/catch）防止单个 handler 崩溃影响其他 handler。 eventbus.js:1-22

player.js 和 playlist.js 均为纯 re-export + bus 监听的薄层，模块边界清晰。 player.js:1-48

问题1：state.js 中 state 是全局可变对象，无任何访问保护，任何模块均可直接修改任意字段，缺乏封装。 state.js:61-94

问题2：actions.js 中 boot 函数使用 setTimeout(..., 50) 魔法数字延迟触发全量加载，这是脆弱的时序依赖。 actions.js:161-168

问题3：subtitle.go 中字幕默认语言硬编码为 "zh" / "字幕"，国际化不友好。 subtitle.go:163-169