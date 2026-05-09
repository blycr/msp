# 归档文档说明

此目录包含不再活跃使用但具有历史参考价值的文档。

---

## 归档文档清单

### 临时性工作指南

| 文档 | 说明 | 归档原因 |
|------|------|----------|
| `CODEX_35_HIGH_VALUE_WORK_CN.md` | AI 协作指南 | 特定工具的工作方式，不属于核心项目文档 |
| `HOME_PERSONAL_REVIEW.md` | 个人场景评审 | 一次性评估报告 |

### 功能测试指南

| 文档 | 说明 | 归档原因 |
|------|------|----------|
| `PIN_TESTING.md` | PIN 认证功能测试指南 | 功能已稳定，测试流程已整合到常规测试中 |

### 发布总结

| 文档 | 说明 | 归档原因 |
|------|------|----------|
| `v0.7.0_RELEASE_SUMMARY.md` | v0.7.0 发布总结 | 与 `docs/release/v0.7.0.md` 内容重复 |

### 快速指南（已整合）

| 文档 | 说明 | 归档原因 |
|------|------|----------|
| `SECURITY_QUICKSTART.md` | 安全功能快速开始 | 内容已整合到主文档 [SECURITY.md](../SECURITY.md) |

### 一次性分析与计划

| 文档 | 说明 | 归档原因 |
|------|------|----------|
| `BUILD_AND_INTERACTION_ANALYSIS.md` | amd64/x64 架构命名问题调查 | Bug 已修复，分析报告归档 |
| `REFACTOR_PLAN.md` | 架构重构计划（5 阶段） | 计划已全部执行，v1.0.0 发布 |
| `frontend-architecture-audit.md` | 前端架构审计报告 | 一次性审计，问题已记录 |
| `OPTIMIZATION.md` | 项目优化分析 | 建议已在 v1.1.3 中全部实现 |
| `SECURITY_AUDIT_REPORT.md` | 安全审计报告 | 审计工作已完成，问题已修复 |
| `DOCUMENTATION_CLEANUP_REPORT.md` | 文档整理报告 | 整理工作已完成 |
| `TRANSCODING_ANALYSIS.md` | 媒体播放架构分析与改进方案 | 4 阶段方案已在 v1.2.0 全部实施 |
| `SECURITY_UPDATE.md` | 安全功能更新说明 | 内容已整合到 [SECURITY.md](../SECURITY.md) |

---

## 当前有效文档

### 核心文档（docs/ 根目录）

| 文档 | 说明 |
|------|------|
| `API_REFERENCE.md` | API 接口参考文档 |
| `CONFIG_EXAMPLE.md` | 配置文件示例（带详细注释） |
| `SECURITY.md` | 安全配置指南 |
| `TRANSCODING.md` | 转码技术文档（FFmpeg、播放策略、硬件加速） |
| `CodeMap.md` | 代码架构导航图 |

### 发布说明

| 目录 | 说明 |
|------|------|
| `docs/release/` | 所有版本的官方发布说明（v0.4.0 ~ v1.2.0） |

### 项目根目录核心文档

| 文档 | 说明 |
|------|------|
| `README.md` / `README_CN.md` | 项目介绍和快速开始 |
| `CHANGELOG.md` | 变更日志 |
| `CONTRIBUTING.md` | 贡献指南 |
| `CODE_OF_CONDUCT.md` | 行为准则 |
| `LICENSE` | MIT 许可证 |
| `config.example.json` | 配置示例 |

---

## 文档分类导航

### 用户文档
- [README.md](../../README.md) - 项目概览和快速开始
- [CONFIG_EXAMPLE.md](../CONFIG_EXAMPLE.md) - 配置参考
- [SECURITY.md](../SECURITY.md) - 安全配置

### 开发者文档
- [API_REFERENCE.md](../API_REFERENCE.md) - API 文档
- [CodeMap.md](../CodeMap.md) - 代码架构导航
- [TRANSCODING.md](../TRANSCODING.md) - 转码技术文档
- [CONTRIBUTING.md](../../CONTRIBUTING.md) - 贡献指南

### 发布历史
- [CHANGELOG.md](../../CHANGELOG.md) - 变更日志
- [docs/release/](../release/) - 详细发布说明

---

## 使用建议

1. **新用户入门**：从 [README.md](../../README.md) 开始
2. **配置服务**：参考 [CONFIG_EXAMPLE.md](../CONFIG_EXAMPLE.md)
3. **安全配置**：阅读 [SECURITY.md](../SECURITY.md)
4. **开发贡献**：查看 [CONTRIBUTING.md](../../CONTRIBUTING.md) 和 [CodeMap.md](../CodeMap.md)
5. **历史参考**：可查阅本归档目录中的文档

---

**归档日期**: 2026-05-09
**最后更新**: 2026-05-09（v1.2.0 归档 TRANSCODING_ANALYSIS.md、SECURITY_UPDATE.md）
**维护状态**: 这些文档不再更新，仅作为历史记录保存
