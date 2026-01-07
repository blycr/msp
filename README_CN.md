# MSP

![GitHub release (latest by date)](https://img.shields.io/github/v/release/blycr/msp)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/blycr/msp)
![GitHub license](https://img.shields.io/github/license/blycr/msp)
![GitHub repo size](https://img.shields.io/github/repo-size/blycr/msp)

一个快速、重视隐私的局域网媒体分享与播放工具：单个可执行文件即可运行，网页端管理共享目录，打开即看即听。

## 为什么用 MSP

- 不用上传网盘：文件留在本机，通过局域网直接在浏览器里看/听
- 不用折腾重型媒体库：不需要复杂部署，启动即可分享目录
- 不用装客户端：同网段手机/电脑打开链接就能用
- 大库也好逛：分类 + 搜索 + 播放列表，适合海量文件浏览
- 播放体验优先：内置播放器与图片预览，提供容器/编码提示，支持字幕/歌词
- 隐私友好：运行时配置只在本地保存，对外分享用示例配置模板

## 快速开始

运行可执行文件（默认端口 `8099`）：

```bash
./bin/windows/x64/msp-windows-amd64.exe
```

启动后访问日志里打印的地址（如 `http://127.0.0.1:8099/`）。

## 配置说明

MSP 使用 JSON 配置文件（首次运行会自动生成 `config.json`），核心配置如下：

### 基础配置
- **port**: 监听端口（默认 `8099`）
- **maxItems**: 最大扫描文件数
  - `0`: **推荐**。无限制全量扫描，配合 SQLite 数据库实现增量更新，首次扫描稍慢，后续极快。
  - `>0` (如 `8000`): 限制扫描数量，适合只想快速预览部分文件的场景（**注意**：达到限制后会停止扫描剩余文件）。
- **shares**: 共享目录列表
  ```json
  "shares": [
    { "label": "电影", "path": "D:\\Movies" },
    { "label": "音乐", "path": "E:\\Music" }
  ]
  ```

### 数据库与增量扫描
从 v0.6.0 起，MSP 引入 SQLite 作为元数据存储：
- **增量更新**: 只有文件变动（修改时间/大小）或新文件才会被重新处理，大幅降低重复启动时的扫描开销。
- **持久化**: 扫描结果存放在 `msp.db`，重启程序无需重新扫描整个硬盘。
- **读取优化**: 即使库中有数十万文件，前端列表加载与搜索也能保持毫秒级响应。

### 黑名单 (Blacklist)
可通过后缀名、文件名、文件夹名或文件大小过滤不需要的内容：
```json
"blacklist": {
  "extensions": [".txt", ".exe"],
  "filenames": ["Thumbs.db"],
  "folders": ["$RECYCLE.BIN", "System Volume Information"],
  "sizeRule": "<1MB" // 排除小于 1MB 的文件
}
```

## 文档与帮助

关于配置、构建、常见问题排查与高级用法，请查阅项目 Wiki：

👉 **[MSP Project Wiki](https://github.com/blycr/msp/wiki)**

## 开源许可

本项目采用 [MIT License](LICENSE) 开源。

## 致谢

- [Plyr](https://github.com/sampotts/plyr) - A simple, accessible and customizable media player
