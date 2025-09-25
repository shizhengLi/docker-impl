package performance

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Optimizer struct {
	pool              *WorkerPool
	imageCache        *ImageCache
	containerCache    *ContainerCache
	metrics           *MetricsCollector
	monitor           *PerformanceMonitor
	prefetchManager   *PrefetchManager
	config            *OptimizerConfig
	mu                sync.RWMutex
}

type OptimizerConfig struct {
	MaxWorkers          int           `json:"max_workers"`
	WorkerIdleTimeout   time.Duration `json:"worker_idle_timeout"`
	ImageCacheSize      int           `json:"image_cache_size"`
	ContainerCacheSize  int           `json:"container_cache_size"`
	PrefetchWorkers     int           `json:"prefetch_workers"`
	EnableMetrics       bool          `json:"enable_metrics"`
	EnableCaching       bool          `json:"enable_caching"`
	EnablePrefetch      bool          `json:"enable_prefetch"`
	GCThreshold         int           `json:"gc_threshold"`
	MemoryLimitPercent  float64       `json:"memory_limit_percent"`
}

var (
	defaultConfig = OptimizerConfig{
		MaxWorkers:         runtime.NumCPU() * 2,
		WorkerIdleTimeout:  30 * time.Second,
		ImageCacheSize:     100,
		ContainerCacheSize: 200,
		PrefetchWorkers:    2,
		EnableMetrics:      true,
		EnableCaching:      true,
		EnablePrefetch:     true,
		GCThreshold:        100,
		MemoryLimitPercent: 80.0,
	}
	optimizer     *Optimizer
	optimizerOnce sync.Once
)

func GetOptimizer() *Optimizer {
	optimizerOnce.Do(func() {
		optimizer = NewOptimizer(&defaultConfig)
	})
	return optimizer
}

func NewOptimizer(config *OptimizerConfig) *Optimizer {
	if config == nil {
		config = &defaultConfig
	}

	opt := &Optimizer{
		config:          config,
		pool:           NewWorkerPool(config.MaxWorkers, config.WorkerIdleTimeout),
		imageCache:     NewImageCache(),
		containerCache: NewContainerCache(),
		metrics:        GetMetrics(),
		monitor:        NewPerformanceMonitor(),
		prefetchManager: NewPrefetchManager(config.PrefetchWorkers),
	}

	if config.EnableMetrics {
		opt.startMetricsCollection()
	}

	if config.EnablePrefetch {
		opt.prefetchManager.Start()
	}

	opt.startGCMonitor()

	logrus.Info("Performance optimizer initialized")
	return opt
}

func (o *Optimizer) startMetricsCollection() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := o.monitor.GetSystemStats()
				logrus.WithField("stats", stats).Debug("System performance stats")
			}
		}
	}()
}

func (o *Optimizer) startGCMonitor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				o.checkMemoryUsage()
			}
		}
	}()
}

func (o *Optimizer) checkMemoryUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get system memory (simplified)
	sysMemory := memStats.Sys
	usedMemory := memStats.Alloc
	usagePercent := float64(usedMemory) / float64(sysMemory) * 100

	if usagePercent > o.config.MemoryLimitPercent {
		logrus.Warnf("Memory usage high: %.2f%%, triggering GC", usagePercent)
		runtime.GC()

		// Clear caches if memory is still high
		runtime.ReadMemStats(&memStats)
		usedMemory = memStats.Alloc
		usagePercent = float64(usedMemory) / float64(sysMemory) * 100

		if usagePercent > o.config.MemoryLimitPercent {
			logrus.Warnf("Memory still high after GC: %.2f%%, clearing caches", usagePercent)
			o.imageCache.Clear()
			o.containerCache.Clear()
		}
	}
}

func (o *Optimizer) OptimizeContainerStart(imageID string, startFunc func() error) error {
	timer := o.monitor.StartTimer(imageID)
	defer timer.Stop(true)

	// Prefetch image if enabled
	if o.config.EnablePrefetch {
		o.prefetchManager.PrefetchImage(imageID)
	}

	// Use worker pool for container start
	var err error
	workerErr := make(chan error, 1)

	work := func() {
		workerErr <- startFunc()
	}

	if err := o.pool.Submit(work); err != nil {
		logrus.Errorf("Failed to submit container start work: %v", err)
		return err
	}

	select {
	case err := <-workerErr:
		if err != nil {
			timer.Stop(false)
			return err
		}
		return nil

	case <-time.After(5 * time.Minute):
		timer.Stop(false)
		return fmt.Errorf("container start timeout")
	}
}

func (o *Optimizer) OptimizeImagePull(imageID string, pullFunc func() error) error {
	startTime := time.Now()

	// Check cache first
	if cachedConfig, found := o.imageCache.GetConfig(imageID); found {
		logrus.Infof("Using cached config for image: %s", imageID)
		return nil
	}

	err := pullFunc()
	duration := time.Since(startTime)

	if err == nil {
		o.metrics.RecordImagePull(imageID, duration)
	}

	return err
}

func (o *Optimizer) GetWorkerPoolStats() map[string]interface{} {
	return o.pool.GetStats()
}

func (o *Optimizer) GetCacheStats() map[string]interface{} {
	cacheStats := make(map[string]interface{})

	if o.config.EnableCaching {
		cacheStats["image_cache"] = map[string]interface{}{
			"hit_rate": o.imageCache.GetHitRate(),
			"size":     o.imageCache.layers.Size(),
		}
	}

	if o.config.EnablePrefetch {
		prefetchStats := o.prefetchManager.GetCacheStats()
		cacheStats["prefetch"] = prefetchStats
	}

	return cacheStats
}

func (o *Optimizer) Stop() {
	if o.config.EnablePrefetch {
		o.prefetchManager.Stop()
	}
	o.pool.Stop()
	logrus.Info("Performance optimizer stopped")
}

type WorkerPool struct {
	workers    []*Worker
	taskQueue  chan Task
	stopChan   chan struct{}
	wg         sync.WaitGroup
	maxWorkers int
	timeout    time.Duration
}

type Task func()

type Worker struct {
	id        int
	taskQueue chan Task
	stopChan  chan struct{}
	timeout   time.Duration
	wg        *sync.WaitGroup
}

func NewWorkerPool(maxWorkers int, timeout time.Duration) *WorkerPool {
	pool := &WorkerPool{
		taskQueue:  make(chan Task, 1000),
		stopChan:   make(chan struct{}),
		maxWorkers: maxWorkers,
		timeout:    timeout,
	}

	pool.start()
	return pool
}

func (p *WorkerPool) start() {
	for i := 0; i < p.maxWorkers; i++ {
		worker := &Worker{
			id:        i,
			taskQueue: p.taskQueue,
			stopChan:  p.stopChan,
			timeout:   p.timeout,
			wg:        &p.wg,
		}
		p.workers = append(p.workers, worker)
		p.wg.Add(1)
		go worker.start()
	}
}

func (p *WorkerPool) Submit(task Task) error {
	select {
	case p.taskQueue <- task:
		return nil
	default:
		return fmt.Errorf("worker pool task queue full")
	}
}

func (p *WorkerPool) Stop() {
	close(p.stopChan)
	p.wg.Wait()
	logrus.Info("Worker pool stopped")
}

func (p *WorkerPool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"max_workers":   p.maxWorkers,
		"queue_length":  len(p.taskQueue),
		"active_workers": len(p.workers),
	}
}

func (w *Worker) start() {
	defer w.wg.Done()

	logrus.Debugf("Worker %d started", w.id)

	for {
		select {
		case task := <-w.taskQueue:
			logrus.Debugf("Worker %d executing task", w.id)
			task()
			logrus.Debugf("Worker %d completed task", w.id)

		case <-w.stopChan:
			logrus.Debugf("Worker %d stopped", w.id)
			return
		}
	}
}