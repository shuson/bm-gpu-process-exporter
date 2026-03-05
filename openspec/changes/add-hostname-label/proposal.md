# Change: 在 GPU 进程指标中添加 hostname 标签

## Why

bm-gpu-process-exporter 部署在 GPU 集群的每台节点上，当 Prometheus 从多台机器抓取指标时，若指标本身不携带节点标识，运维人员只能依赖 Prometheus 的 `instance` 标签（来自 scrape target 配置）来区分来源机器。这种方式要求 Prometheus 配置与实际主机名保持一致，且在 Grafana 查询时不够直观。

在指标中直接嵌入 `hostname` 标签，可以让每条时序数据自带节点身份，无需依赖外部 relabel 配置，便于跨节点对比和告警规则编写。

## What Changes

- 在 `gpu_process_info`、`gpu_process_memory_used_megabytes`、`gpu_process_utilization_percent` 三个 GaugeVec 指标中新增 `hostname` 标签
- 启动时通过 `os.Hostname()` 获取本机主机名，作为静态值注入所有指标标签
- 支持通过环境变量 `HOSTNAME_OVERRIDE` 覆盖自动检测的主机名（适用于容器化部署场景）
- `exporter_updates_total`、`exporter_update_errors_total`、`nvidia_smi_up` 等运维类指标**不**添加 hostname 标签（这些指标通常通过 Prometheus target label 区分来源）

## Impact

- Affected specs: `metrics-labels`（新建能力）
- Affected code: `cmd/exporter/main.go`
  - 指标定义处新增 `hostname` 标签维度
  - `updateGPUProcesses` 函数签名需传入 hostname 参数
  - `main()` 函数启动时解析 hostname
- **BREAKING**：现有 Prometheus 查询、告警规则、Grafana Dashboard 中针对上述三个指标的查询需要适配新增的 `hostname` 标签（可使用 `{hostname=~".*"}` 兼容）
