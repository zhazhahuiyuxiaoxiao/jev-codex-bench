# 对照实验基线

## Purpose

依据 `cmd/jev-bench/main.go`、`internal/bench/bench.go`、`internal/bench/bench_test.go` 记录现有实验流程，不代表已取得实验效果。

## Requirements

### Requirement: 固定任务与统一模型

运行器 SHALL 对选定公开任务顺序执行三组，并要求显式统一 Codex 模型；无效任务 MUST 在启动实验前拒绝。

#### Scenario: 默认运行
- **WHEN** 三个默认任务、模型、Key、工具与输出目录条件满足
- **THEN** 按 baseline、prefilter、autonomous 的已定义实验组执行各任务，并逐次保存结果

#### Scenario: 未提供模型
- **WHEN** CodexModel 为空
- **THEN** 返回必须指定统一模型的错误，不启动实验

#### Scenario: 已有结果
- **WHEN** 输出目录存在 results.json
- **THEN** 拒绝覆盖并要求使用新输出目录

### Requirement: 隐藏验收与结果真实性

运行器 SHALL 在 Codex 完成后注入隐藏测试，并区分 pass、fail 与 infra_error；诊断保存失败不得改变测试结论。

#### Scenario: 隐藏测试成功
- **WHEN** Codex 有完成事件且注入后的测试成功
- **THEN** 记录 pass 与 TestsPassed 为 true

#### Scenario: 隐藏测试失败
- **WHEN** 隐藏测试非超时失败
- **THEN** 记录 fail，并尝试保留失败输出；诊断失败另记 diagnostics_error

#### Scenario: Codex 未完成
- **WHEN** 子进程失败、超时或未产生完成事件
- **THEN** 记录 infra_error，不把结果称为成功
