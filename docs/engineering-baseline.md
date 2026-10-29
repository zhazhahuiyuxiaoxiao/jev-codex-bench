# jev-codex-bench 工程基线

- 核对日期：2026-10-29。
- 基线：`main@366af6cc7810e967df468d0fdb6deacefe28a28b`（文档补录前提交）。
- 来源：本仓库已提交源码、测试和现有说明；不纳入其他工作区未提交或未合并能力。
- 范围：只补文档和流程，不改变业务、依赖、数据或部署。

## 能力与证据

| 验收目标 | 源码 | 已有测试 |
| --- | --- | --- |
| 三任务/三实验组、独立临时副本、注入隐藏验证器 | `internal/bench/bench.go` 的 `RunAll`、`runOne`、`CheckFixtures` | `internal/bench/bench_test.go`；`fixtures/`、`_validators/` |
| 输出目录保护、结果/诊断分离、JSONL 使用量解析 | `internal/bench/bench.go` | `bench_test.go` 的解析、诊断、验收文件同名隔离用例 |
| 请求前预算预留、失败保留预留 | `internal/jev/ledger.go`、`client.go` | `internal/jev/client_test.go` |
| 只排序固定公开文件、跳过符号链接、最多两条推荐路径 | `internal/selector/selector.go`、`internal/broker/broker.go` | `selector_test.go`、`broker_test.go` |
| MCP 仅提供无文本输入的 rank_files | `internal/server/server.go` | `internal/server/server_test.go` |

## 启动与验证

Go 1.25.0（`go.mod`）与 Git 是测试/诊断前置条件；依赖须已准备，首次下载另行授权。`go test ./...` 使用模拟边界，`go run ./cmd/jev-bench check-fixtures` 在临时副本核验可见测试通过、加入隐藏测试后应失败；并发样本使用 race 检查，需要对应工具链支持。它们不调用真实 Jev/Codex，当前文档任务没有执行。

真实实验入口为 `go run ./cmd/jev-bench run-all --codex-model <AVAILABLE_MODEL_ID> --out results/<NEW_RUN>`；此为授权后使用的说明，不是本次已执行命令。需要当前可用 Codex CLI/登录和 `TYPESAFE_API_KEY=<KEY>` 环境变量；程序要求统一模型、使用新输出目录，默认九次 Codex，单任务为三次。本次没有读取账户、Key、账本或实验结果。

自主组 MCP 经权限受限 Unix socket 请求运行器排序固定公开原样本；预算文件使用 `syscall.Flock`，不能未经验证宣称原生 Windows 可运行。CLI 参数兼容性和账号模型可用性应在真实实验前检查，README 中模型名称仅为历史示例。

## 验收清单与边界

- [ ] 验证样本可见测试成功、修复前隐藏测试失败，区分工具链错误与预期断言失败。
- [ ] 预算超限不发请求，失败保留预留，缺 Key 不启动实验。
- [ ] TypeSafe Key 不进入子进程；符号链接和超额文件不成为模型输入。
- [ ] 诊断文件与隐藏测试结果分别保留；自主组不调用工具时记为零。
- [ ] 获授权后才运行真实实验，复核 CLI 兼容性、价格、账户额度与输出隐私。

费率是代码常量而非动态账单；本地 $0.50 账本只限制该账本的 Jev 请求，不能控制 Codex 用量或服务商账户总消费。三个小样本单次结果不能推广为普遍提升。本次不验证模型效果或发布任何结果。仓库无附加 LICENSE，公开可见不等于授予复用许可。

## 本次验证记录

- 已执行：OpenSpec 1.14.2 全量严格格式校验、`git diff --check`、新改 Markdown 相对路径核对及命令来源静态核对，结果见本次 PR。
- 未执行：业务测试、依赖安装、服务启动、真实平台集成、收费模型调用、发布/部署。仅文档变更不将源码审阅表述为运行通过。
- 后续功能验收应运行上文对应测试，并补正常、边界和失败路径证据；外部系统与真实设备需另行授权及验证。

## 文档分工

[README](../README.md) 是用途与启动入口；[AGENTS](../AGENTS.md) 约束修改流程；[OpenSpec](../openspec/README.md) 区分当前能力与未来变更。旧文档中的已验证结论保留其原时间与上下文，不作为当前生产证据。
