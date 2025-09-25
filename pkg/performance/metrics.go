package performance

import (
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type MetricsCollector struct {
	containerStartTime    *prometheus.HistogramVec
	imagePullTime         *prometheus.HistogramVec
	memoryUsage           *prometheus.GaugeVec
	cpuUsage              *prometheus.GaugeVec
	diskIO                *prometheus.CounterVec
	networkIO             *prometheus.CounterVec
	activeContainers      *prometheus.Gauge
	activeImages          *prometheus.Gauge
	containerStartCounter *prometheus.CounterVec
}

var (
	metrics     *MetricsCollector
	metricsOnce sync.Once
)

func GetMetrics() *MetricsCollector {
	metricsOnce.Do(func() {
		metrics = &MetricsCollector{
			containerStartTime: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "mydocker_container_start_time_seconds",
					Help: "Time taken to start containers",
					Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
				},
				[]string{"image", "status"},
			),
			imagePullTime: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "mydocker_image_pull_time_seconds",
					Help: "Time taken to pull images",
					Buckets: []float64{1.0, 5.0, 10.0, 30.0, 60.0, 300.0},
				},
				[]string{"image"},
			),
			memoryUsage: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "mydocker_memory_usage_bytes",
					Help: "Memory usage by containers",
				},
				[]string{"container", "type"},
			),
			cpuUsage: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "mydocker_cpu_usage_percent",
					Help: "CPU usage by containers",
				},
				[]string{"container"},
			),
			diskIO: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "mydocker_disk_io_bytes_total",
					Help: "Disk I/O bytes total",
				},
				[]string{"container", "operation"},
			),
			networkIO: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "mydocker_network_io_bytes_total",
					Help: "Network I/O bytes total",
				},
				[]string{"container", "direction"},
			),
			activeContainers: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "mydocker_active_containers",
					Help: "Number of active containers",
				},
			),
			activeImages: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "mydocker_active_images",
					Help: "Number of active images",
				},
			),
			containerStartCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "mydocker_container_starts_total",
					Help: "Total number of container starts",
				},
				[]string{"image", "result"},
			),
		}

		prometheus.MustRegister(
			metrics.containerStartTime,
			metrics.imagePullTime,
			metrics.memoryUsage,
			metrics.cpuUsage,
			metrics.diskIO,
			metrics.networkIO,
			metrics.activeContainers,
			metrics.activeImages,
			metrics.containerStartCounter,
		)
	})
	return metrics
}

func (m *MetricsCollector) RecordContainerStart(image string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	m.containerStartTime.WithLabelValues(image, status).Observe(duration.Seconds())

	result := "success"
	if !success {
		result = "failed"
	}
	m.containerStartCounter.WithLabelValues(image, result).Inc()

	if success {
		m.activeContainers.Inc()
	}
}

func (m *MetricsCollector) RecordImagePull(image string, duration time.Duration) {
	m.imagePullTime.WithLabelValues(image).Observe(duration.Seconds())
	m.activeImages.Inc()
}

func (m *MetricsCollector) UpdateContainerMetrics(containerID string, memoryUsage uint64, cpuUsage float64) {
	m.memoryUsage.WithLabelValues(containerID, "rss").Set(float64(memoryUsage))
	m.cpuUsage.WithLabelValues(containerID).Set(cpuUsage)
}

func (m *MetricsCollector) RecordDiskIO(containerID string, readBytes, writeBytes uint64) {
	m.diskIO.WithLabelValues(containerID, "read").Add(float64(readBytes))
	m.diskIO.WithLabelValues(containerID, "write").Add(float64(writeBytes))
}

func (m *MetricsCollector) RecordNetworkIO(containerID string, rxBytes, txBytes uint64) {
	m.networkIO.WithLabelValues(containerID, "rx").Add(float64(rxBytes))
	m.networkIO.WithLabelValues(containerID, "tx").Add(float64(txBytes))
}

func (m *MetricsCollector) ContainerStopped(containerID string) {
	m.activeContainers.Dec()
	m.memoryUsage.DeleteLabelValues(containerID, "rss")
	m.cpuUsage.DeleteLabelValues(containerID)
}

func (m *MetricsCollector) ImageRemoved() {
	m.activeImages.Dec()
}

type PerformanceMonitor struct {
	startTime time.Time
	metrics   *MetricsCollector
}

func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		startTime: time.Now(),
		metrics:   GetMetrics(),
	}
}

func (p *PerformanceMonitor) GetSystemStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"uptime_seconds": time.Since(p.startTime).Seconds(),
		"goroutines":     runtime.NumGoroutine(),
		"memory_stats": map[string]interface{}{
			"allocated_bytes": memStats.Alloc,
			"total_allocated": memStats.TotalAlloc,
			"system_memory":   memStats.Sys,
			"num_gc":          memStats.NumGC,
			"gc_cpu_fraction": memStats.GCCPUFraction,
		},
	}
}

func (p *PerformanceMonitor) StartTimer(image string) *ContainerTimer {
	return &ContainerTimer{
		image:     image,
		startTime: time.Now(),
		metrics:   p.metrics,
	}
}

type ContainerTimer struct {
	image     string
	startTime time.Time
	metrics   *MetricsCollector
}

func (t *ContainerTimer) Stop(success bool) {
	duration := time.Since(t.startTime)
	t.metrics.RecordContainerStart(t.image, duration, success)
	logrus.Infof("Container start time: %v, success: %v", duration, success)
}

func LogPerformanceMetrics(operation string, duration time.Duration, additionalInfo map[string]interface{}) {
	logrus.WithFields(logrus.Fields{
		"operation":      operation,
		"duration_ms":    duration.Milliseconds(),
		"additional_info": additionalInfo,
	}).Info("Performance metric")
}