# Change: Add GPU UUID Metric

## Why

当前 exporter 仅通过 GPU 索引（`gpu` 标签，如 `0`、`1`）标识 GPU，但 GPU 索引在驱动重载或系统重启后可能发生变化。GPU UUID 是 NVIDIA 为每块 GPU 分配的全局唯一标识符，跨重启稳定不变，是在多节点集群中精确追踪特定 GPU 硬件的可靠依据。

## What Changes

- 新增 `gpu_info` GaugeVec 指标，标签为 `hostname`、`gpu`（索引）、`uuid`，值恒为 1，作为 GPU 硬件元数据
- 复用现有 `getUUIDToIndexMap()` 函数（已通过 `nvidia-smi --query-gpu=index,uuid` 获取 UUID），在每次更新周期中同步填充该指标
- `gpu_info` 指标在每次 `updateFromNvidiaSMI` 调用时 Reset 并重新填充，与进程级指标保持一致的生命周期

## Impact

- Affected specs: `gpu-metrics`（新建）
- Affected code: `cmd/exporter/main.go`
  - `main()`：注册 `gpu_info` GaugeVec，传入 `updateFromNvidiaSMI`
  - `updateFromNvidiaSMI()`：接收 `gpu_info` 参数，调用填充逻辑
  - `updateGPUInfo()`（新函数）：遍历 UUID→Index 映射，填充 `gpu_info` 指标
- 无破坏性变更：现有指标标签和语义不变
