package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	host := flag.String("host", getEnv("HOST", "0.0.0.0"), "HTTP listen host")
	port := flag.String("port", getEnv("PORT", "9101"), "HTTP listen port")
	updateEvery := flag.Int("update-interval-seconds", getEnvAsInt("UPDATE_INTERVAL_SECONDS", 5), "Metric refresh interval in seconds")
	flag.Parse()

	// 获取本机主机名，支持通过 HOSTNAME_OVERRIDE 环境变量覆盖（适用于容器化部署）
	hostname := getEnv("HOSTNAME_OVERRIDE", "")
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			log.Printf("warning: failed to get hostname: %v, using 'unknown'", err)
			hostname = "unknown"
		}
	}

	gpuProcessInfo := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_process_info",
			Help: "Metadata for active GPU processes. Always 1 while process is active.",
		},
		[]string{"hostname", "gpu", "uuid", "pid", "user", "program"},
	)

	gpuProcessMemoryUsedMB := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_process_memory_used_megabytes",
			Help: "GPU memory used by process in MiB.",
		},
		[]string{"hostname", "gpu", "uuid", "pid", "user", "program"},
	)

	gpuProcessUtilization := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_process_utilization_percent",
			Help: "Estimated GPU utilization percentage per active GPU process.",
		},
		[]string{"hostname", "gpu", "uuid", "pid", "user", "program"},
	)

	exporterUpdates := promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "exporter_updates_total",
			Help: "Total number of internal metric update cycles.",
		},
	)

	exporterUpdateErrors := promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "exporter_update_errors_total",
			Help: "Total number of internal metric update failures.",
		},
	)

	nvidiaSMIUp := promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nvidia_smi_up",
			Help: "Whether nvidia-smi query succeeds (1) or fails (0).",
		},
	)

	go func() {
		ticker := time.NewTicker(time.Duration(*updateEvery) * time.Second)
		defer ticker.Stop()

		for {
			if err := updateFromNvidiaSMI(gpuProcessInfo, gpuProcessMemoryUsedMB, gpuProcessUtilization, hostname); err != nil {
				nvidiaSMIUp.Set(0)
				exporterUpdateErrors.Inc()
				log.Printf("nvidia-smi update failed: %v", err)
			} else {
				nvidiaSMIUp.Set(1)
			}
			exporterUpdates.Inc()
			<-ticker.C
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := net.JoinHostPort(*host, *port)
	log.Printf("exporter listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	return val
}

func getEnvAsInt(key string, fallback int) int {
	val := getEnv(key, strconv.Itoa(fallback))
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

func updateFromNvidiaSMI(
	gpuProcessInfo *prometheus.GaugeVec,
	gpuProcessMemoryUsedMB *prometheus.GaugeVec,
	gpuProcessUtilization *prometheus.GaugeVec,
	hostname string,
) error {
	cmd := exec.Command(
		"nvidia-smi",
		"--query-gpu=index,utilization.gpu",
		"--format=csv,noheader,nounits",
	)

	out, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
		return fmt.Errorf("no GPU rows returned by nvidia-smi")
	}

	gpuUtilByIndex := make(map[string]float64, len(lines))

	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			return fmt.Errorf("unexpected nvidia-smi row format: %q", line)
		}

		gpuIndex := strings.TrimSpace(fields[0])
		utilGPU, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if err != nil {
			return fmt.Errorf("parse utilization.gpu for %q: %w", gpuIndex, err)
		}

		gpuUtilByIndex[gpuIndex] = utilGPU
	}

	if err := updateGPUProcesses(gpuProcessInfo, gpuProcessMemoryUsedMB, gpuProcessUtilization, gpuUtilByIndex, hostname); err != nil {
		return err
	}

	return nil
}

func updateGPUProcesses(
	gpuProcessInfo, gpuProcessMemoryUsedMB, gpuProcessUtilization *prometheus.GaugeVec,
	gpuUtilByIndex map[string]float64,
	hostname string,
) error {
	uuidToIndex, err := getUUIDToIndexMap()
	if err != nil {
		return err
	}

	// 构造反向映射：GPU 索引 → UUID
	indexToUUID := make(map[string]string, len(uuidToIndex))
	for uuid, index := range uuidToIndex {
		indexToUUID[index] = uuid
	}

	cmd := exec.Command(
		"nvidia-smi",
		"--query-compute-apps=gpu_uuid,pid,process_name,used_gpu_memory",
		"--format=csv,noheader,nounits",
	)

	out, err := cmd.Output()
	if err != nil {
		return err
	}

	gpuProcessInfo.Reset()
	gpuProcessMemoryUsedMB.Reset()
	gpuProcessUtilization.Reset()

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}

	processCountByGPU := make(map[string]int)
	type processSample struct {
		gpuIndex string
		uuid     string
		pid      string
		user     string
		program  string
		memUsed  float64
	}
	var processSamples []processSample

	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 4 {
			return fmt.Errorf("unexpected nvidia-smi process row format: %q", line)
		}

		gpuUUID := strings.TrimSpace(fields[0])
		pid := strings.TrimSpace(fields[1])
		processName := strings.TrimSpace(fields[2])
		memUsedRaw := strings.TrimSpace(fields[3])

		gpuIndex, ok := uuidToIndex[gpuUUID]
		if !ok {
			gpuIndex = gpuUUID
		}
		uuid := gpuUUID

		username, program := lookupPIDUserProgram(pid)
		if program == "" {
			program = processName
		}
		if username == "" {
			username = "unknown"
		}

		memUsedMB, err := strconv.ParseFloat(memUsedRaw, 64)
		if err != nil {
			memUsedMB = 0
		}

		processCountByGPU[gpuIndex]++
		processSamples = append(processSamples, processSample{
			gpuIndex: gpuIndex,
			uuid:     uuid,
			pid:      pid,
			user:     username,
			program:  program,
			memUsed:  memUsedMB,
		})
	}

	for _, sample := range processSamples {
		gpuProcessInfo.WithLabelValues(hostname, sample.gpuIndex, sample.uuid, sample.pid, sample.user, sample.program).Set(1)
		gpuProcessMemoryUsedMB.WithLabelValues(hostname, sample.gpuIndex, sample.uuid, sample.pid, sample.user, sample.program).Set(sample.memUsed)

		processCount := processCountByGPU[sample.gpuIndex]
		if processCount <= 0 {
			continue
		}
		// nvidia-smi 不直接暴露每进程 GPU 利用率，按活跃进程数均分作为估算值
		perProcessUtil := gpuUtilByIndex[sample.gpuIndex] / float64(processCount)
		gpuProcessUtilization.WithLabelValues(hostname, sample.gpuIndex, sample.uuid, sample.pid, sample.user, sample.program).Set(perProcessUtil)
	}

	return nil
}

func getUUIDToIndexMap() (map[string]string, error) {
	cmd := exec.Command(
		"nvidia-smi",
		"--query-gpu=index,uuid",
		"--format=csv,noheader,nounits",
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		idx := strings.TrimSpace(fields[0])
		uuid := strings.TrimSpace(fields[1])
		result[uuid] = idx
	}

	return result, nil
}

func lookupPIDUserProgram(pid string) (string, string) {
	cmd := exec.Command("ps", "-p", pid, "-o", "user=", "-o", "args=")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", ""
	}

	fields := strings.Fields(line)
	if len(fields) < 1 {
		return "", ""
	}

	user := fields[0]
	args := strings.TrimSpace(strings.TrimPrefix(line, user))
	if args == "" {
		return user, ""
	}

	program := buildProgramLabel(args)
	return user, program
}

func buildProgramLabel(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}

	exe := filepath.Base(fields[0])
	lowerExe := strings.ToLower(exe)
	if isPythonExecutable(lowerExe) {
		return buildPythonProgramLabel(exe, fields[1:])
	}

	switch lowerExe {
	case "torchrun", "deepspeed", "horovodrun":
		return buildLauncherProgramLabel(exe, fields[1:], map[string]bool{
			"--nnodes":         true,
			"--nproc-per-node": true,
			"--node-rank":      true,
			"--master-addr":    true,
			"--master-port":    true,
			"--rdzv-backend":   true,
			"--rdzv-endpoint":  true,
			"--rdzv-id":        true,
			"--max-restarts":   true,
			"-n":               true,
			"-np":              true,
			"-H":               true,
			"-x":               true,
		})
	case "accelerate":
		launcher := exe
		rest := fields[1:]
		if len(rest) > 0 && rest[0] == "launch" {
			launcher = launcher + " launch"
			rest = rest[1:]
		}
		return buildLauncherProgramLabel(launcher, rest, map[string]bool{
			"--config-file":       true,
			"--num-processes":     true,
			"--num-machines":      true,
			"--machine-rank":      true,
			"--main-process-ip":   true,
			"--main-process-port": true,
			"--mixed-precision":   true,
			"--dynamo-backend":    true,
			"-m":                  true,
		})
	case "mpirun", "mpiexec", "srun":
		return buildLauncherProgramLabel(exe, fields[1:], map[string]bool{
			"-n":                true,
			"-np":               true,
			"-N":                true,
			"-H":                true,
			"-x":                true,
			"-w":                true,
			"-mca":              true,
			"--host":            true,
			"--hosts":           true,
			"--ntasks":          true,
			"--nodes":           true,
			"--nproc-per-node":  true,
			"--gpus-per-task":   true,
			"--cpus-per-task":   true,
			"--ntasks-per-node": true,
		})
	case "sglang", "sglang-router", "sglang_router", "sglang-launch-server":
		return buildCommandWithSubcommandLabel(exe, fields[1:], map[string]bool{
			"--model-path":        true,
			"--model":             true,
			"--host":              true,
			"--port":              true,
			"--tp-size":           true,
			"--dp-size":           true,
			"--pp-size":           true,
			"--quantization":      true,
			"--max-num-seqs":      true,
			"--max-model-len":     true,
			"--served-model-name": true,
		})
	case "llamafactory", "llamafactory-cli":
		return buildCommandWithSubcommandLabel(exe, fields[1:], map[string]bool{
			"--config":             true,
			"--dataset":            true,
			"--model_name":         true,
			"--model_name_or_path": true,
			"--template":           true,
			"--stage":              true,
			"--finetuning_type":    true,
		})
	case "tritonserver", "trtllm-serve", "trtllm-build", "trtllm-bench":
		return buildCommandWithSubcommandLabel(exe, fields[1:], map[string]bool{
			"--model-repository": true,
			"--grpc-port":        true,
			"--http-port":        true,
			"--model":            true,
			"--engine_dir":       true,
			"--tokenizer_dir":    true,
		})
	}

	return exe
}

func isPythonExecutable(lowerExe string) bool {
	return strings.HasPrefix(lowerExe, "python")
}

func buildPythonProgramLabel(exe string, args []string) string {
	if len(args) >= 2 && args[0] == "-m" {
		return exe + " -m " + args[1]
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasSuffix(arg, ".py") || strings.HasSuffix(arg, ".pyc") {
			return exe + " " + filepath.Base(arg)
		}
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return exe + " " + filepath.Base(arg)
	}

	return exe
}

func buildLauncherProgramLabel(launcher string, args []string, optionsWithValue map[string]bool) string {
	if len(args) == 0 {
		return launcher
	}

	idx := firstLaunchedCommandIndex(args, optionsWithValue)
	if idx >= len(args) {
		return launcher
	}

	command := args[idx]
	commandBase := filepath.Base(command)
	commandLower := strings.ToLower(commandBase)
	commandArgs := args[idx+1:]

	if isPythonExecutable(commandLower) {
		return launcher + " " + buildPythonProgramLabel(commandBase, commandArgs)
	}

	if strings.HasSuffix(commandLower, ".py") || strings.HasSuffix(commandLower, ".pyc") {
		return launcher + " " + filepath.Base(command)
	}

	return launcher + " " + commandBase
}

func buildCommandWithSubcommandLabel(command string, args []string, optionsWithValue map[string]bool) string {
	if len(args) == 0 {
		return command
	}

	idx := firstLaunchedCommandIndex(args, optionsWithValue)
	if idx >= len(args) {
		return command
	}

	subcommand := filepath.Base(args[idx])
	return command + " " + subcommand
}

func firstLaunchedCommandIndex(args []string, optionsWithValue map[string]bool) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return i + 1
			}
			return len(args)
		}

		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "=") {
				continue
			}
			if optionsWithValue[arg] && i+1 < len(args) {
				i++
			}
			continue
		}

		return i
	}

	return len(args)
}
