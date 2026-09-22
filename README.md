# Jev × Codex Bench

一个可复现的小样本实验：在相同的 Go 编码任务上，比较普通 Codex、Jev 预筛选上下文、以及 Codex 自主决定是否调用 Jev 的结果。它不是新的通用 Jev MCP 连接器，也不预设 Jev 一定有收益。

本项目目前只有三个自制、可公开的任务：CSV 公式注入、缓存 TTL 边界、并发任务领取。每个任务有可见测试和实验结束后才加入的隐藏验收测试；“隐藏”仅指实验中的 Codex 看不到，GitHub 读者可以检查 `_validators/`。公开源码不包含用户工作仓库、生产配置或私人日志。

## 当前状态与费用

- 只运行 `go test ./...` 或 `check-fixtures` 不会调用 Jev 或 Codex，不消耗 API 额度。
- `run-all` 会运行 **3 个任务 × 3 个实验组 = 9 次 Codex**，其中 Jev 组会请求 TypeSafe。Codex 用量与 Jev 额度是两套独立计量；本项目只限制 Jev 请求，不限制或估算 Codex 订阅/API 用量。
- Jev 模型固定为 `jev-1.13.0`，按当前官方标价 **$0.042 / 百万输入 token** 估算。`.local/jev-budget.json` 是本地共享账本：每次请求前按请求字节数预留保守金额，总预留加实耗不能超过 **$0.50**；成功后以 API 返回的 `usage.input_tokens` 记实耗。失败或响应不完整时保留预留金额，因为不能确认服务端是否计费。一次请求的真实账单或未来调价仍以 TypeSafe 控制台为准；本地保护不是服务商账户级硬限额。[TypeSafe 模型价格](https://docs.typesafe.ai/models) · [API 参考](https://docs.typesafe.ai/api)
- Key 只由实验运行器从启动进程的 `TYPESAFE_API_KEY` 环境变量读取，不传入任何 Codex 子进程或 MCP 子进程，也不写入仓库或 Codex 配置。自主组通过权限受限的本地 Unix socket 向运行器请求文件排序；MCP 工具不接收文本参数，运行器只会把预设的公开任务与仓库中原始样本文件送往 Jev，绝不扫描 Codex 可写的临时工作区。不要把 Key 发到聊天、粘进 shell 历史、截图或提交到 Git。`.env.example` 只列变量名，不存值。

## 准备与离线验证

需要 Go 1.25+、已登录的 Codex CLI，以及正式运行时可用的 TypeSafe API Key。依赖中只有官方 MCP Go SDK 为直接依赖；TypeSafe HTTP 请求用 Go 标准库实现。初次 `go mod download` / `go test` 可能需要下载 Go 依赖。

```sh
go test ./...
go run ./cmd/jev-bench check-fixtures
```

第二条命令应说明三个任务的可见测试通过、隐藏测试在修复前失败；这是实验样本有效性的基本检查。`go test ./...` 还覆盖预算拒绝、请求失败时保留预留额、Jev 响应解析、文件排序、MCP 交互和 Codex JSONL 解析。不要直接在 `fixtures/` 内运行 `_validators`：运行器会在临时副本中加入它们。

## 运行真实实验

先在 TypeSafe 控制台确认赠送额度和自动充值状态；无需向本仓库或聊天提供 Key。在你的 macOS zsh 终端中交互输入 Key（输入时不回显，也不出现在命令历史中），然后选择当前 Codex 账号可用的**同一个**模型 ID，例如：

```sh
read -rs 'TYPESAFE_API_KEY?TypeSafe API Key: '
echo
export TYPESAFE_API_KEY
go run ./cmd/jev-bench run-all --codex-model gpt-5.6-sol
unset TYPESAFE_API_KEY
```

运行器按顺序启动九次 `codex exec`，每次都把公开任务复制进单独的临时目录，用 `--sandbox workspace-write`、`--ephemeral` 和 `--json` 记录结果。它忽略用户级 Codex 配置，仅为自主组注入项目自带的本地 MCP 工具；Key 不进入 Codex 进程。运行后输出 `results/results.json`、`results/report.md`；额度账本固定在 `.local/jev-budget.json`，换输出目录也不会重置本仓库的预算。中断后已经完成的记录仍在原输出目录；重新运行需要新输出目录，且继续共用预算账本。Codex 官方文档说明 JSONL 中 `turn.completed.usage` 的 token 字段，以及本地 STDIO MCP 配置。[OpenAI Docs：非交互模式](https://learn.chatgpt.com/docs/non-interactive-mode) · [OpenAI Docs：MCP](https://learn.chatgpt.com/docs/extend/mcp)

报告记录每次隐藏测试是否通过、总耗时、Codex 输入/输出 token、MCP 调用次数、Jev 调用和实耗。自主组若选择不调用 MCP，会如实记录为 0 次；不能把它算成“Jev 帮助”。测试失败或环境错误也会单列，不会被折算成成功。原始 Codex 对话和提交差异默认不保存，避免公开时顺手泄漏上下文。

## 实验解释与安全边界

首轮每个任务每组只跑一次，指标仅供观察，不足以宣称普遍、统计显著的效率提升。三组的任务、模型和 Codex 权限保持一致，但提示词和工具可用性正是实验变量。预筛选组用 Jev 的 Noul 判断对仓库中的 Go 文件排序，给 Codex 前两个文件名；自主组由 Codex 决定要不要调用相同排序工具。Jev 分数只是线索，最终以隐藏测试验收。

工具只扫描给定公开样本目录内最多 12 个常规 `.go` 文件，不跟随符号链接，每个文件最多取前 1,800 字节；每次 Jev 请求最大 16 KiB。MCP 服务器仅暴露 `rank_files`，没有写文件、执行命令或访问其他工作仓库的能力。它只连接运行器创建的本地代理，而不能直接取得 TypeSafe Key。Codex 本身在隔离样本目录里有写入权限以完成任务，但仍是外部模型，请先确认你接受九次 Codex 运行及其独立用量。

本仓库已公开在 GitHub，但程序没有自动发布、提交、推送、部署、付款或自动充值功能。真实实验结果和本地额度账本不会上传：`.gitignore` 排除了 `results/` 与 `.local/`。当前没有附加开源许可证；公开可见不等于已授权他人复制、修改或再分发，若希望开放复用需另行选择许可证。
