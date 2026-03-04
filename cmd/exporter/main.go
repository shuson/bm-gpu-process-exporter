package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
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

	gpuUtilization := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_utilization_percent",
			Help: "Current utilization percentage per GPU (0-100).",
		},
		[]string{"gpu"},
	)

	gpuMemoryUtilization := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_memory_utilization_percent",
			Help: "Current memory controller utilization percentage per GPU (0-100).",
		},
		[]string{"gpu"},
	)

	gpuProcessInfo := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_process_info",
			Help: "Metadata for active GPU processes. Always 1 while process is active.",
		},
		[]string{"gpu", "pid", "user", "program"},
	)

	gpuProcessMemoryUsedMB := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_process_memory_used_megabytes",
			Help: "GPU memory used by process in MiB.",
		},
		[]string{"gpu", "pid", "user", "program"},
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
			if err := updateFromNvidiaSMI(gpuUtilization, gpuMemoryUtilization, gpuProcessInfo, gpuProcessMemoryUsedMB); err != nil {
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
	gpuUtilization *prometheus.GaugeVec,
	gpuMemoryUtilization *prometheus.GaugeVec,
	gpuProcessInfo *prometheus.GaugeVec,
	gpuProcessMemoryUsedMB *prometheus.GaugeVec,
) error {
	cmd := exec.Command(
		"nvidia-smi",
		"--query-gpu=index,utilization.gpu,utilization.memory",
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

	gpuUtilization.Reset()
	gpuMemoryUtilization.Reset()

	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			return fmt.Errorf("unexpected nvidia-smi row format: %q", line)
		}

		gpuIndex := strings.TrimSpace(fields[0])
		utilGPU, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if err != nil {
			return fmt.Errorf("parse utilization.gpu for %q: %w", gpuIndex, err)
		}
		utilMem, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		if err != nil {
			return fmt.Errorf("parse utilization.memory for %q: %w", gpuIndex, err)
		}

		gpuUtilization.WithLabelValues(gpuIndex).Set(utilGPU)
		gpuMemoryUtilization.WithLabelValues(gpuIndex).Set(utilMem)
	}

	if err := updateGPUProcesses(gpuProcessInfo, gpuProcessMemoryUsedMB); err != nil {
		return err
	}

	return nil
}

func updateGPUProcesses(gpuProcessInfo, gpuProcessMemoryUsedMB *prometheus.GaugeVec) error {
	uuidToIndex, err := getUUIDToIndexMap()
	if err != nil {
		return err
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

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}

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

		gpuProcessInfo.WithLabelValues(gpuIndex, pid, username, program).Set(1)
		gpuProcessMemoryUsedMB.WithLabelValues(gpuIndex, pid, username, program).Set(memUsedMB)
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
	cmd := exec.Command("ps", "-p", pid, "-o", "user=", "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", ""
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", ""
	}

	user := fields[0]
	program := strings.Join(fields[1:], " ")
	return user, program
}
