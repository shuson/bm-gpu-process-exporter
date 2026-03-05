## 1. 指标定义更新

- [x] 1.1 在 `main()` 中调用 `os.Hostname()` 获取主机名，并支持 `HOSTNAME_OVERRIDE` 环境变量覆盖
- [x] 1.2 在 `gpuProcessInfo`、`gpuProcessMemoryUsedMB`、`gpuProcessUtilization` 三个 GaugeVec 的 label 列表中追加 `"hostname"`
- [x] 1.3 更新 `updateFromNvidiaSMI` 和 `updateGPUProcesses` 函数签名，接受 `hostname string` 参数
- [x] 1.4 在 `WithLabelValues(...)` 调用处追加 `hostname` 参数

## 2. 验证

- [x] 2.1 编译通过：`go build ./cmd/exporter/`
- [ ] 2.2 在有 GPU 的机器上运行并 `curl localhost:9101/metrics`，确认三个指标均含 `hostname` 标签且值正确
- [ ] 2.3 验证 `HOSTNAME_OVERRIDE=custom-node ./exporter` 时，指标中 hostname 值为 `custom-node`
- [ ] 2.4 验证 `exporter_updates_total` 等运维指标**不含** hostname 标签
