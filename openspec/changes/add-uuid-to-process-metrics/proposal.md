# Proposal: add-uuid-to-process-metrics

## Summary

在三个进程级指标（`gpu_process_info`、`gpu_process_memory_used_megabytes`、`gpu_process_utilization_percent`）的标签集中直接增加 `uuid` 标签，使每条时间序列同时携带 GPU 索引和 GPU UUID，无需额外的 `gpu_info` 元数据指标。

## Motivation

当前进程级指标仅通过 `gpu` 标签（GPU 索引，如 `"0"`、`"1"`）标识 GPU。GPU 索引在驱动重载或机器重启后可能发生变化，而 GPU UUID 是 NVIDIA 分配的全局唯一标识符，跨重启稳定。用户希望在进程指标中直接读取 UUID，无需通过 join 查询关联。

## Current Behaviour

```
gpu_process_info{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang"} 1
gpu_process_memory_used_megabytes{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang"} 40960
gpu_process_utilization_percent{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang"} 12.5
```

## Desired Behaviour

```
gpu_process_info{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang",uuid="GPU-a67ec542-4591-36d7-37e8-b4c35981a140"} 1
gpu_process_memory_used_megabytes{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang",uuid="GPU-a67ec542-4591-36d7-37e8-b4c35981a140"} 40960
gpu_process_utilization_percent{gpu="0",hostname="h20node12",pid="63636",program="python3.11 sft.py",user="lxmxiang",uuid="GPU-a67ec542-4591-36d7-37e8-b4c35981a140"} 12.5
```

## Out of Scope

- 不新增 `gpu_info` 独立指标（由 `add-gpu-uuid-metric` 提案引入，该提案将被废弃或回滚）
- 不修改 `hostname`、`gpu`、`pid`、`user`、`program` 等现有标签

## Implementation Approach

1. 将三个进程级 GaugeVec 的标签列表从 `["hostname", "gpu", "pid", "user", "program"]` 改为 `["hostname", "gpu", "uuid", "pid", "user", "program"]`
2. `updateGPUProcesses()` 中已有 `uuidToIndex` 映射（`uuid → index`），构造反向映射 `indexToUUID`，在填充指标时传入 `uuid` 值
3. 移除 `gpu_info` GaugeVec 及相关的 `updateGPUInfo()` 函数（`add-gpu-uuid-metric` 引入的代码）

## Breaking Change

**是**：现有 Prometheus 查询、告警规则、Grafana 面板中引用这三个指标的 label matcher 需要更新（新增 `uuid` 标签不影响无 `uuid` 过滤条件的查询，但指标的 label set 发生变化，已有 recording rules 需重新评估）。

## Dependencies

- 依赖 `getUUIDToIndexMap()` 函数（已存在）
- 与 `add-gpu-uuid-metric` 提案冲突，实现本提案后应废弃 `add-gpu-uuid-metric`
