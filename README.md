# bm-gpu-process-exporter

Prometheus exporter in Go that collects GPU utilization and GPU process metadata from `nvidia-smi`.

## Features

- Exposes `/metrics` for Prometheus scraping
- Exposes `/healthz` for liveness checks
- Collects per-process metrics for GPU users:
  - `gpu_process_info{gpu,pid,user,program} 1`
  - `gpu_process_memory_used_megabytes{gpu,pid,user,program}`
  - `gpu_process_utilization_percent{gpu,pid,user,program}` (estimated as per-GPU utilization split evenly across active processes on that GPU)
- Exporter self-observability:
  - `exporter_updates_total`
  - `exporter_update_errors_total`
  - `nvidia_smi_up`

## Requirements

- Go 1.22+
- Linux host with NVIDIA drivers and `nvidia-smi` in `PATH`

## Usage

### 1. Run exporter

```bash
go mod tidy
go run ./cmd/exporter
```

Exporter defaults:

- Address: `0.0.0.0:9101`
- Update interval: `5s`

Run with custom host/port:

```bash
go run ./cmd/exporter --host 127.0.0.1 --port 9200
```

### 2. Check endpoints

- Metrics: `http://localhost:9101/metrics`
- Health: `http://localhost:9101/healthz`

### 3. Configure Prometheus scrape

```yaml
scrape_configs:
  - job_name: bm-gpu-task-exporter
    static_configs:
      - targets: ["localhost:9101"]
```

## Configuration

- Program arguments:
  - `--host` (default: `0.0.0.0`)
  - `--port` (default: `9101`)
  - `--update-interval-seconds` (default: `5`)
- Environment fallback (used as argument defaults):
  - `HOST`
  - `PORT`
  - `UPDATE_INTERVAL_SECONDS`

Example:

```bash
go run ./cmd/exporter --host 0.0.0.0 --port 9200 --update-interval-seconds 2
```

## Development Guide

### Local development

```bash
go mod tidy
go fmt ./...
go build ./...
go run ./cmd/exporter
```

### Build Linux amd64 binary (Ubuntu target)

```bash
mkdir -p bin /tmp/gocache /tmp/gopath
GOCACHE=/tmp/gocache GOPATH=/tmp/gopath go mod tidy
GOCACHE=/tmp/gocache GOPATH=/tmp/gopath CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o bin/bm-gpu-task-exporter-linux-amd64 ./cmd/exporter
```

### Validate runtime dependencies

Run on the target host:

```bash
nvidia-smi --query-gpu=index,utilization.gpu,utilization.memory --format=csv,noheader,nounits
nvidia-smi --query-compute-apps=gpu_uuid,pid,process_name,used_gpu_memory --format=csv,noheader,nounits
```

If those commands work, exporter metrics should populate on each update cycle.

## Troubleshooting

- `nvidia_smi_up 0`: exporter cannot query `nvidia-smi` (binary missing, no driver, or permissions issue).
- `exporter_update_errors_total` increasing: parse/query failures from `nvidia-smi`.
- No `gpu_process_*` metrics: no active compute processes on GPU at the moment.
