## ADDED Requirements

### Requirement: Hostname Label on Process Metrics

导出器 SHALL 在所有 GPU 进程级别指标（`gpu_process_info`、`gpu_process_memory_used_megabytes`、`gpu_process_utilization_percent`）中包含 `hostname` 标签，其值为本机主机名。

#### Scenario: 默认使用系统主机名

- **WHEN** 导出器启动且未设置 `HOSTNAME_OVERRIDE` 环境变量
- **THEN** 三个 GPU 进程指标的 `hostname` 标签值等于 `os.Hostname()` 返回的主机名

#### Scenario: 通过环境变量覆盖主机名

- **WHEN** 导出器启动时设置了 `HOSTNAME_OVERRIDE` 环境变量（非空值）
- **THEN** 三个 GPU 进程指标的 `hostname` 标签值等于 `HOSTNAME_OVERRIDE` 的值，而非系统主机名

#### Scenario: 运维指标不含 hostname 标签

- **WHEN** Prometheus 抓取 `/metrics` 端点
- **THEN** `exporter_updates_total`、`exporter_update_errors_total`、`nvidia_smi_up` 指标**不**包含 `hostname` 标签
