## 1. 移除 gpu_info 相关代码

- [x] 1.1 删除 `main()` 中 `gpuInfo` GaugeVec 的注册代码
- [x] 1.2 从 `updateFromNvidiaSMI()` 签名中移除 `gpuInfo` 参数，更新调用处
- [x] 1.3 从 `updateGPUProcesses()` 签名中移除 `gpuInfo` 参数，更新调用处
- [x] 1.4 删除 `updateGPUInfo()` 函数

## 2. 三个进程级指标增加 uuid 标签

- [x] 2.1 将 `gpuProcessInfo`、`gpuProcessMemoryUsedMB`、`gpuProcessUtilization` 三个 GaugeVec 的标签列表改为 `["hostname", "gpu", "uuid", "pid", "user", "program"]`
- [x] 2.2 在 `updateGPUProcesses()` 中，利用已有的 `uuidToIndex` 映射构造反向映射 `indexToUUID`（`index → uuid`）
- [x] 2.3 在填充三个指标时，从 `indexToUUID` 中取出对应 `uuid` 值，传入 `WithLabelValues()`；若找不到则使用空字符串

## 3. 验证

- [x] 3.1 编译通过：`go build ./cmd/exporter/`
- [ ] 3.2 在有 GPU 的机器上运行并 `curl localhost:9101/metrics`，确认三个进程级指标均包含 `uuid` 标签，且值与 `nvidia-smi -L` 一致
- [ ] 3.3 确认 `/metrics` 输出中不再出现 `gpu_info` 指标
