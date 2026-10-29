# Jev 输入与预算基线

## Purpose

依据 `internal/jev/ledger.go`、`internal/jev/client.go`、`internal/selector/selector.go`、`internal/server/server.go` 及对应测试。

## Requirements

### Requirement: 请求前预留预算

账本 SHALL 在网络请求前持锁预留金额；支出加预留加本次预留超过固定预算时 MUST 拒绝调用。

#### Scenario: 超过预算
- **WHEN** 预留后总金额将超过 0.50 USD
- **THEN** 返回预算错误，不执行网络回调

#### Scenario: 请求失败
- **WHEN** 网络或响应校验回调失败
- **THEN** 保留预留金额，不假定服务商未收费

#### Scenario: 请求成功
- **WHEN** 返回按使用量和固定费率计算的金额且未超过本次预留
- **THEN** 释放本次预留、累计金额并增加成功调用计数

### Requirement: 有限公开样本输入

排序器 SHALL 仅收集给定样本目录的常规 Go 文件，不跟随符号链接；文件数必须在 1–12，单文件内容最多取 1800 字节。

#### Scenario: 有效样本排序
- **WHEN** 文件与任务输入有效且 Jev 返回有效评分
- **THEN** 按评分降序、同分路径排序，最多返回两个优选路径

#### Scenario: 文件数量越界
- **WHEN** 可收集 Go 文件为零或超过十二个
- **THEN** 排序返回错误，不请求 Jev

#### Scenario: 自主组工具调用
- **WHEN** Codex 调用 rank_files
- **THEN** 工具不接受待上传正文，只经本地 broker 对固定公开原样本与预设任务排序
