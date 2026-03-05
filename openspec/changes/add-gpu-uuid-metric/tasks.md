## 1. 指标定义

- [x] 1.1 在 `main()` 中注册 `gpu_info` GaugeVec，标签为 `["hostname", "gpu", "uuid"]`
- [x] 1.2 将 `gpu_info` 作为参数传入 `updateFromNvidiaSMI()`

## 2. 填充逻辑

- [x] 2.1 新增 `updateGPUInfo(gpuInfo *prometheus.GaugeVec, uuidToIndex map[string]string, hostname string)` 函数，遍历映射并调用 `WithLabelValues(hostname, index, uuid).Set(1)`
- [x] 2.2 在 `updateGPUProcesses()` 中，获取 `uuidToIndex` 映射后立即调用 `updateGPUInfo()`

## 3. 验证

- [x] 3.1 编译通过：`go build ./cmd/exporter/`
- [ ] 3.2 在有 GPU 的机器上运行并 `curl localhost:9101/metrics`，确认输出包含 `gpu_info{hostname="...", gpu="0", uuid="GPU-..."}` 等条目，每块 GPU 一行，值为 `1`（需在有 GPU 的机器上手动验证）
- [ ] 3.3 验证 UUID 值与 `nvidia-smi -L` 输出中的 UUID 一致（需在有 GPU 的机器上手动验证）
