package performance

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	lru "github.com/hashicorp/golang-lru"
)

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Clear()
	Size() int
}

type LRUCache struct {
	cache *lru.Cache
	mu    sync.RWMutex
}

func NewLRUCache(size int) (*LRUCache, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &LRUCache{cache: cache}, nil
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Get(key)
}

func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(key, value)
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Remove(key)
}

func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Purge()
}

func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Len()
}

type ImageCache struct {
	layers      *LRUCache
	manifests   *LRUCache
	configs     *LRUCache
	hits       int
	misses      int
	mu         sync.RWMutex
}

func NewImageCache() *ImageCache {
	layersCache, _ := NewLRUCache(100)
	manifestsCache, _ := NewLRUCache(50)
	configsCache, _ := NewLRUCache(50)

	return &ImageCache{
		layers:    layersCache,
		manifests: manifestsCache,
		configs:   configsCache,
	}
}

func (c *ImageCache) GetLayer(layerID string) (interface{}, bool) {
	value, found := c.layers.Get(layerID)
	c.mu.Lock()
	defer c.mu.Unlock()
	if found {
		c.hits++
	} else {
		c.misses++
	}
	return value, found
}

func (c *ImageCache) SetLayer(layerID string, layer interface{}) {
	c.layers.Set(layerID, layer)
	logrus.Debugf("Cached layer: %s", layerID)
}

func (c *ImageCache) GetManifest(imageID string) (interface{}, bool) {
	return c.manifests.Get(imageID)
}

func (c *ImageCache) SetManifest(imageID string, manifest interface{}) {
	c.manifests.Set(imageID, manifest)
	logrus.Debugf("Cached manifest for image: %s", imageID)
}

func (c *ImageCache) GetConfig(imageID string) (interface{}, bool) {
	return c.configs.Get(imageID)
}

func (c *ImageCache) SetConfig(imageID string, config interface{}) {
	c.configs.Set(imageID, config)
	logrus.Debugf("Cached config for image: %s", imageID)
}

func (c *ImageCache) GetHitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	if total == 0 {
		return 0.0
	}
	return float64(c.hits) / float64(total)
}

func (c *ImageCache) Clear() {
	c.layers.Clear()
	c.manifests.Clear()
	c.configs.Clear()
	c.mu.Lock()
	c.hits = 0
	c.misses = 0
	c.mu.Unlock()
	logrus.Info("Image cache cleared")
}

type ContainerCache struct {
	processes   *LRUCache
	networks    *LRUCache
	stats       *LRUCache
	mu          sync.RWMutex
}

func NewContainerCache() *ContainerCache {
	processesCache, _ := NewLRUCache(200)
	networksCache, _ := NewLRUCache(100)
	statsCache, _ := NewLRUCache(100)

	return &ContainerCache{
		processes: processesCache,
		networks:  networksCache,
		stats:     statsCache,
	}
}

func (c *ContainerCache) GetProcess(containerID string) (interface{}, bool) {
	return c.processes.Get(containerID)
}

func (c *ContainerCache) SetProcess(containerID string, process interface{}) {
	c.processes.Set(containerID, process)
	logrus.Debugf("Cached process info for container: %s", containerID)
}

func (c *ContainerCache) GetNetwork(containerID string) (interface{}, bool) {
	return c.networks.Get(containerID)
}

func (c *NetworkCache) SetNetwork(containerID string, network interface{}) {
	c.networks.Set(containerID, network)
	logrus.Debugf("Cached network info for container: %s", containerID)
}

func (c *ContainerCache) GetStats(containerID string) (interface{}, bool) {
	return c.stats.Get(containerID)
}

func (c *ContainerCache) SetStats(containerID string, stats interface{}) {
	c.stats.Set(containerID, stats)
	logrus.Debugf("Cached stats for container: %s", containerID)
}

func (c *ContainerCache) Clear() {
	c.processes.Clear()
	c.networks.Clear()
	c.stats.Clear()
	logrus.Info("Container cache cleared")
}

type PrefetchManager struct {
	imageCache    *ImageCache
	containerCache *ContainerCache
	prefetchQueue chan string
	workers       int
	stopChan      chan struct{}
}

func NewPrefetchManager(workers int) *PrefetchManager {
	return &PrefetchManager{
		imageCache:    NewImageCache(),
		containerCache: NewContainerCache(),
		prefetchQueue: make(chan string, 100),
		workers:       workers,
		stopChan:      make(chan struct{}),
	}
}

func (p *PrefetchManager) Start() {
	for i := 0; i < p.workers; i++ {
		go p.prefetchWorker(i)
	}
	logrus.Infof("Started %d prefetch workers", p.workers)
}

func (p *PrefetchManager) Stop() {
	close(p.stopChan)
	logrus.Info("Prefetch manager stopped")
}

func (p *PrefetchManager) PrefetchImage(imageID string) {
	select {
	case p.prefetchQueue <- imageID:
		logrus.Debugf("Queued prefetch for image: %s", imageID)
	default:
		logrus.Warnf("Prefetch queue full, skipping prefetch for image: %s", imageID)
	}
}

func (p *PrefetchManager) prefetchWorker(id int) {
	for {
		select {
		case imageID := <-p.prefetchQueue:
			logrus.Debugf("Worker %d prefetching image: %s", id, imageID)
			// Simulate prefetch work
			time.Sleep(100 * time.Millisecond)

			// Cache the prefetched image
			p.imageCache.SetConfig(imageID, map[string]interface{}{
				"prefetched": true,
				"timestamp":   time.Now(),
			})

		case <-p.stopChan:
			logrus.Debugf("Prefetch worker %d stopped", id)
			return
		}
	}
}

func (p *PrefetchManager) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"image_cache": map[string]interface{}{
			"layers_hit_rate":    p.imageCache.GetHitRate(),
			"layers_size":        p.imageCache.layers.Size(),
			"manifests_size":     p.imageCache.manifests.Size(),
			"configs_size":       p.imageCache.configs.Size(),
		},
		"container_cache": map[string]interface{}{
			"processes_size": p.containerCache.processes.Size(),
			"networks_size":  p.containerCache.networks.Size(),
			"stats_size":     p.containerCache.stats.Size(),
		},
		"prefetch_queue": map[string]interface{}{
			"queue_length": len(p.prefetchQueue),
			"workers":      p.workers,
		},
	}
}