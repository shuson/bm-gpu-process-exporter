# Project Context

## Purpose

bm-gpu-process-exporter 是一个 Prometheus 指标导出器，用于从 NVIDIA GPU 收集详细的进程级别 GPU 使用情况。

**核心目标：**
- 收集 GPU 进程的内存使用情况和利用率百分比
- 提供进程元数据（GPU ID、PID、用户名、程序名）
- 智能识别各类深度学习工作负载（PyTorch、DeepSpeed、HuggingFace Accelerate 等）
- 暴露 Prometheus 兼容的 `/metrics` 端点供监控系统抓取

**应用场景：**
- GPU 集群资源监控
- 多进程并行训练任务追踪
- 大模型推理服务监测

## Tech Stack

- **编程语言**: Go 1.23.0 (toolchain 1.23.4)
- **主要依赖**: github.com/prometheus/client_golang v1.23.2
- **HTTP 框架**: Go 标准库 net/http
- **部署方式**: systemd 服务
- **运行环境**: Linux + NVIDIA CUDA 驱动 + nvidia-smi
- **构建方式**: CGO_ENABLED=0 纯静态编译

## Project Conventions

### Code Style

- **格式化**: 使用 `go fmt ./...` 标准格式化
- **函数命名**: camelCase（如 `updateFromNvidiaSMI`、`getEnv`）
- **变量命名**: camelCase（如 `gpuProcessInfo`、`processCountByGPU`）
- **常量/环境变量**: 全大写下划线分隔（如 `UPDATE_INTERVAL_SECONDS`）
- **错误处理**: 显式返回 error，不使用 panic

**Prometheus 指标命名规范：**
- 遵循 Prometheus 官方命名约定
- 使用小写下划线格式：`gpu_process_memory_used_megabytes`
- 标签使用小写：`gpu`、`pid`、`user`、`program`

### Architecture Patterns

- **单文件应用**: 所有核心逻辑在 `cmd/exporter/main.go` 中（约 512 行）
- **轮询架构**: 使用 ticker 定时器每 N 秒调用 nvidia-smi
- **命令执行**: 通过 `os/exec` 执行外部命令（nvidia-smi、ps）
- **解析管道**: CSV 输出 → 字符串分割 → 类型转换 → 指标更新
- **配置优先级**: 命令行参数 > 环境变量 > 默认值

**指标类型使用：**
- GaugeVec: 进程信息、内存使用、利用率
- Counter: 更新计数、错误计数
- Gauge: nvidia-smi 连接状态

### Testing Strategy

- **当前状态**: 无单元测试（待改进）
- **验证方式**: 通过 nvidia-smi 命令和 curl 手动验证
- **推荐方向**: 
  - 添加工作负载标签解析器的单元测试
  - 添加指标输出格式的集成测试

### Git Workflow

- **主分支**: main
- **提交风格**: 描述性英文提交信息
- **版本标签**: 语义化标签（如 alpha_1）
- **分支策略**: 单一主分支开发，feature 分支用于较大改动

**提交信息示例：**
```
Enhance workload label parsing for common launcher commands
Add systemd installer and refine exported GPU process metrics
```

## Domain Context

**GPU 监控领域知识：**

1. **nvidia-smi 查询**：
   - `--query-gpu`: 查询 GPU 级别信息（索引、利用率、内存）
   - `--query-compute-apps`: 查询进程级别信息（PID、内存使用）
   - GPU UUID 用于跨重启的稳定标识

2. **利用率估计**：
   - nvidia-smi 不提供每进程的 GPU 利用率
   - 采用公平分配策略：`per_process_utilization = total_gpu_utilization / active_process_count`

3. **工作负载识别**：
   - Python 工作负载：识别 python/python3 及 `-m` 模块调用
   - 分布式训练框架：torchrun、deepspeed、accelerate、horovodrun
   - MPI 工具：mpirun、mpiexec、srun
   - 推理服务：sglang、tritonserver、trtllm-serve

4. **Prometheus 集成**：
   - `/metrics` 端点暴露所有指标
   - `/healthz` 端点用于健康检查
   - 支持 Prometheus 的 relabel 和 job 配置

## Important Constraints

- **系统依赖**: 必须运行在安装了 NVIDIA CUDA 驱动的 Linux 系统上
- **nvidia-smi 可用性**: 导出器依赖 nvidia-smi 命令行工具
- **权限要求**: 服务用户需要 video 组权限以访问 GPU
- **单机部署**: 每台 GPU 服务器需要独立部署导出器实例

## External Dependencies

**系统依赖：**
- NVIDIA CUDA 驱动（任何支持 nvidia-smi 的版本）
- nvidia-smi 命令行工具
- ps 命令（用于进程信息查询）
- systemd（用于服务管理）

**Go 依赖（主要）：**
- `github.com/prometheus/client_golang` - Prometheus 客户端库
- `github.com/prometheus/common` - Prometheus 通用工具
- `github.com/prometheus/procfs` - Linux /proc 文件系统解析

**监控生态集成：**
- Prometheus Server - 指标抓取和存储
- Grafana - 可视化展示（可选）
