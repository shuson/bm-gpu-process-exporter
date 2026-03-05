## ADDED Requirements

### Requirement: GPU Info Metric
The system SHALL expose a `gpu_info` GaugeVec metric with labels `hostname`, `gpu` (index), and `uuid`, with a constant value of `1`, to provide stable GPU hardware identity metadata.

#### Scenario: GPU info metric populated on each update cycle
- **WHEN** `updateFromNvidiaSMI` is called and `nvidia-smi --query-gpu=index,uuid` succeeds
- **THEN** `gpu_info{hostname="<host>", gpu="<index>", uuid="<UUID>"}` is set to `1` for every detected GPU

#### Scenario: GPU info metric reset before repopulation
- **WHEN** a new update cycle begins
- **THEN** `gpu_info` is Reset before being repopulated, so stale entries from removed GPUs are cleared

#### Scenario: UUID sourced from nvidia-smi query
- **WHEN** the exporter queries GPU metadata
- **THEN** the `uuid` label value matches the UUID returned by `nvidia-smi --query-gpu=index,uuid --format=csv,noheader,nounits` (e.g., `GPU-a67ec542-4591-36d7-37e8-b4c35981a140`)

#### Scenario: GPU info metric absent when nvidia-smi fails
- **WHEN** `nvidia-smi` command returns a non-zero exit code
- **THEN** `gpu_info` is not updated and `nvidia_smi_up` is set to `0`
