## MODIFIED Requirements

### Requirement: gpu_process_info 指标携带 uuid 标签

**描述**：`gpu_process_info` 指标的标签集中 MUST 增加 `uuid` 标签，值为该 GPU 的 NVIDIA UUID（格式 `GPU-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`）。

#### Scenario: 正常进程输出包含 uuid

- **Given** 机器上有 GPU 进程在运行
- **When** 抓取 `/metrics` 端点
- **Then** `gpu_process_info` 的每条时间序列包含 `uuid` 标签，值与 `nvidia-smi -L` 输出中对应 GPU 的 UUID 一致

#### Scenario: uuid 与 gpu 索引对应正确

- **Given** GPU 0 的 UUID 为 `GPU-a67ec542-4591-36d7-37e8-b4c35981a140`
- **When** GPU 0 上有进程运行
- **Then** 该进程对应的 `gpu_process_info` 时间序列中 `gpu="0"` 且 `uuid="GPU-a67ec542-4591-36d7-37e8-b4c35981a140"`

---

### Requirement: gpu_process_memory_used_megabytes 指标携带 uuid 标签

**描述**：`gpu_process_memory_used_megabytes` 指标的标签集中 MUST 增加 `uuid` 标签，与 `gpu_process_info` 保持一致。

#### Scenario: 内存指标包含 uuid

- **Given** 机器上有 GPU 进程在运行
- **When** 抓取 `/metrics` 端点
- **Then** `gpu_process_memory_used_megabytes` 的每条时间序列包含 `uuid` 标签

---

### Requirement: gpu_process_utilization_percent 指标携带 uuid 标签

**描述**：`gpu_process_utilization_percent` 指标的标签集中 MUST 增加 `uuid` 标签，与 `gpu_process_info` 保持一致。

#### Scenario: 利用率指标包含 uuid

- **Given** 机器上有 GPU 进程在运行
- **When** 抓取 `/metrics` 端点
- **Then** `gpu_process_utilization_percent` 的每条时间序列包含 `uuid` 标签

---

## REMOVED Requirements

### Requirement: gpu_info 独立元数据指标

**描述**：移除由 `add-gpu-uuid-metric` 引入的 `gpu_info` GaugeVec 及 `updateGPUInfo()` 函数。UUID 信息已直接内嵌到进程级指标中，无需独立指标。
