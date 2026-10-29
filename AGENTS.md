# jev-codex-bench 项目约定

## 文档与变更约定（2026-10-29）

- `fixtures/` 是故意保留缺陷的公开样本，`_validators/` 是实验后注入的验收测试；不得把样本缺陷当成常规 bug 顺手修掉。
- 保持任务、模型和权限对照，明确提示与工具可用性是实验变量；报告失败、环境错误和零 MCP 调用，不宣传统计显著收益。
- 不读取 `.local/`、`results/` 或真实密钥；不得把私人源码变成 Jev 输入，不得将 TYPESAFE_API_KEY 传入 Codex/MCP 子进程。
- 验证命令为 `go test ./...` 和 `go run ./cmd/jev-bench check-fixtures`。`run-all` 会启动真实 Codex 与收费 Jev，必须另获明确实验/用量授权，不能作为文档校验自动执行。

- 先读 README、[工程基线](docs/engineering-baseline.md)及相关源码/测试；当前规范位于 `openspec/specs/`。
- 先明确可验证验收目标；涉及权限、持久化格式、外部传输、删除或失败语义的业务变化，先说明旧/新行为并确认范围。
- 当前文档基线不授权部署、迁移、提交、推送或读取真实凭据/用户数据。示例使用占位符，保留来源与许可。
- 非简单行为变更先按 [OpenSpec 流程](openspec/README.md)建立 change；仅文档修正保持最小范围，不伪造功能提案。
- 文档检查：`OPENSPEC_TELEMETRY=0 openspec validate --all --strict --no-interactive`、`git diff --check`，并核对相对链接及命令来源；格式检查不等于业务验收。
