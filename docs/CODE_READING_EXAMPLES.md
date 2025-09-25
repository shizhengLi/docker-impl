# 代码阅读示例和练习 - 边学边练

## 🎯 学习目标

本文档通过具体的代码示例和练习，帮助你深入理解MyDocker项目的源码。每个例子都配有详细的讲解和实践练习。

## 📝 准备工作

### 1. 设置开发环境
```bash
# 确保你有Go 1.21+环境
go version

# 安装依赖（如果需要）
go mod tidy

# 运行基础测试，确保环境正常
go test ./pkg/store -v
```

### 2. 推荐工具
- **代码编辑器**: VS Code 或 GoLand
- **调试工具**: Delve (Go调试器)
- **Git**: 版本控制，方便对比代码变更

---

## 📚 示例1：理解数据类型和存储系统

### 1.1 阅读目标
理解镜像和容器的数据结构，以及数据持久化的实现。

### 1.2 关键文件
- `pkg/types/image.go`
- `pkg/types/container.go`
- `pkg/store/store.go`

### 1.3 代码示例分析

#### 示例1.1：理解镜像数据结构

```go
// pkg/types/image.go
type Image struct {
    ID       string      `json:"id"`      // 唯一标识符
    Name     string      `json:"name"`     // 镜像名称
    Tag      string      `json:"tag"`      // 版本标签
    Layers   []string    `json:"layers"`   // 文件系统层
    Config   ImageConfig `json:"config"`   // 运行配置
    Size     int64       `json:"size"`     // 镜像大小
    Created  string      `json:"created"`   // 创建时间
}

type ImageConfig struct {
    Cmd        []string          `json:"cmd"`        // 默认命令
    Entrypoint []string          `json:"entrypoint"` // 入口点
    Env        []string          `json:"env"`        // 环境变量
    WorkingDir string            `json:"working_dir"` // 工作目录
    ExposedPorts map[string]struct{} `json:"exposed_ports"` // 暴露端口
}
```

**练习1.1.1**：创建一个新的镜像结构
```go
// 在main函数中添加以下代码
func main() {
    // 创建一个Nginx镜像实例
    nginxImage := &types.Image{
        ID:      "sha256:1234567890abcdef",
        Name:    "nginx",
        Tag:     "1.21",
        Size:    142000000, // 142MB
        Created: "2023-01-01T00:00:00Z",
        Config: types.ImageConfig{
            Cmd:        []string{"nginx", "-g", "daemon off;"},
            ExposedPorts: map[string]struct{}{
                "80/tcp": {},
                "443/tcp": {},
            },
        },
    }

    // 打印镜像信息
    fmt.Printf("镜像名称: %s:%s\n", nginxImage.Name, nginxImage.Tag)
    fmt.Printf("镜像ID: %s\n", nginxImage.ID[:12])
    fmt.Printf("镜像大小: %.2f MB\n", float64(nginxImage.Size)/1024/1024)
    fmt.Printf("暴露端口: %v\n", nginxImage.Config.ExposedPorts)
}
```

#### 示例1.2：理解容器数据结构

```go
// pkg/types/container.go
type Container struct {
    ID         string           `json:"id"`         // 容器ID
    Name       string           `json:"name"`       // 容器名称
    ImageID    string           `json:"image_id"`   // 基础镜像ID
    Status     ContainerStatus  `json:"status"`     // 容器状态
    CreatedAt  string           `json:"created_at"` // 创建时间
    StartedAt  string           `json:"started_at"` // 启动时间
    FinishedAt string           `json:"finished_at"`// 结束时间
    Config     ContainerConfig  `json:"config"`     // 容器配置
}

type ContainerConfig struct {
    Image      string            `json:"image"`      // 镜像名称
    Command    []string          `json:"command"`    // 运行命令
    Env        []string          `json:"env"`        // 环境变量
    Resources  ResourceConfig    `json:"resources"`  // 资源限制
    Network    NetworkConfig     `json:"network"`    // 网络配置
}
```

**练习1.2.1**：创建一个基于镜像的容器
```go
func createContainerFromImage(image *types.Image, containerName string) *types.Container {
    return &types.Container{
        ID:        generateContainerID(),
        Name:      containerName,
        ImageID:   image.ID,
        Status:    types.StatusCreated,
        CreatedAt: time.Now().Format(time.RFC3339),
        Config: types.ContainerConfig{
            Image:   fmt.Sprintf("%s:%s", image.Name, image.Tag),
            Command: image.Config.Cmd,
            Resources: types.ResourceConfig{
                CPU:    0.5, // 0.5个CPU核心
                Memory: 512 * 1024 * 1024, // 512MB内存
            },
        },
    }
}

func generateContainerID() string {
    return fmt.Sprintf("container-%x", time.Now().UnixNano())[:12]
}
```

#### 示例1.3：理解存储系统

```go
// pkg/store/store.go
type Store struct {
    dataDir string // 数据存储目录
}

func (s *Store) Save(key string, value interface{}) error {
    // 1. 验证参数
    if key == "" {
        return fmt.Errorf("key cannot be empty")
    }

    // 2. 序列化数据
    data, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %w", err)
    }

    // 3. 确保目录存在
    if err := os.MkdirAll(s.dataDir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    // 4. 构建文件路径
    path := filepath.Join(s.dataDir, key+".json")

    // 5. 写入文件
    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }

    return nil
}

func (s *Store) Get(key string, value interface{}) error {
    // 1. 构建文件路径
    path := filepath.Join(s.dataDir, key+".json")

    // 2. 读取文件
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }

    // 3. 反序列化数据
    if err := json.Unmarshal(data, value); err != nil {
        return fmt.Errorf("failed to unmarshal data: %w", err)
    }

    return nil
}
```

**练习1.3.1**：使用存储系统保存和读取数据
```go
func storageExample() {
    // 创建临时存储目录
    tempDir := "/tmp/mydocker-test"
    store := store.NewStore(tempDir)

    // 创建镜像
    image := &types.Image{
        ID:   "test-image-123",
        Name: "test-image",
        Tag:  "latest",
    }

    // 保存镜像
    if err := store.Save(image.ID, image); err != nil {
        fmt.Printf("保存镜像失败: %v\n", err)
        return
    }

    // 读取镜像
    var retrievedImage types.Image
    if err := store.Get(image.ID, &retrievedImage); err != nil {
        fmt.Printf("读取镜像失败: %v\n", err)
        return
    }

    fmt.Printf("原始镜像: %+v\n", image)
    fmt.Printf("读取镜像: %+v\n", retrievedImage)

    // 清理测试数据
    os.RemoveAll(tempDir)
}
```

### 1.4 实战练习

**练习1.4.1**：扩展数据结构
在`pkg/types/container.go`中添加以下字段：
```go
type Container struct {
    // ... 现有字段 ...
    Labels       map[string]string `json:"labels"`       // 容器标签
    Annotations  map[string]string `json:"annotations"`  // 容器注释
    RestartCount int             `json:"restart_count"` // 重启次数
}
```

**练习1.4.2**：实现存储List功能
在`pkg/store/store.go`中添加：
```go
func (s *Store) List(prefix string) ([]string, error) {
    entries, err := os.ReadDir(s.dataDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory: %w", err)
    }

    var keys []string
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }

        filename := entry.Name()
        if strings.HasSuffix(filename, ".json") {
            key := strings.TrimSuffix(filename, ".json")
            if prefix == "" || strings.HasPrefix(key, prefix) {
                keys = append(keys, key)
            }
        }
    }

    return keys, nil
}
```

---

## 📚 示例2：理解镜像管理

### 2.1 阅读目标
理解镜像管理的CRUD操作，以及镜像生命周期。

### 2.2 关键文件
- `pkg/image/manager.go`

### 2.3 代码示例分析

#### 示例2.1：理解镜像管理器结构

```go
// pkg/image/manager.go
type Manager struct {
    store *store.Store // 依赖注入：存储系统
}

func NewManager(store *store.Store) *Manager {
    return &Manager{
        store: store,
    }
}
```

#### 示例2.2：理解拉取镜像逻辑

```go
func (m *Manager) PullImage(name, tag string) (*Image, error) {
    // 1. 验证参数
    if name == "" {
        return nil, fmt.Errorf("image name cannot be empty")
    }

    // 2. 设置默认tag
    if tag == "" {
        tag = "latest"
    }

    imageID := generateImageID(name, tag)

    // 3. 检查镜像是否已存在
    var existing Image
    if err := m.store.Get(imageID, &existing); err == nil {
        return nil, fmt.Errorf("image %s:%s already exists", name, tag)
    }

    // 4. 创建镜像对象
    image := &Image{
        ID:      imageID,
        Name:    name,
        Tag:     tag,
        Created: time.Now().Format(time.RFC3339),
        Size:    calculateImageSize(name, tag), // 模拟计算大小
    }

    // 5. 保存镜像
    if err := m.store.Save(image.ID, image); err != nil {
        return nil, fmt.Errorf("failed to save image: %w", err)
    }

    logrus.Infof("Successfully pulled image %s:%s", name, tag)
    return image, nil
}
```

#### 示例2.3：理解列出镜像逻辑

```go
func (m *Manager) ListImages() ([]*Image, error) {
    // 1. 获取所有镜像键
    keys, err := m.store.List("image-")
    if err != nil {
        return nil, fmt.Errorf("failed to list images: %w", err)
    }

    // 2. 读取所有镜像
    var images []*Image
    for _, key := range keys {
        var image Image
        if err := m.store.Get(key, &image); err != nil {
            logrus.Warnf("Failed to get image %s: %v", key, err)
            continue
        }
        images = append(images, &image)
    }

    // 3. 按创建时间排序
    sort.Slice(images, func(i, j int) bool {
        return images[i].Created > images[j].Created
    })

    return images, nil
}
```

### 2.4 实战练习

**练习2.4.1**：实现镜像标签功能
```go
func (m *Manager) TagImage(sourceImageID, targetName, targetTag string) error {
    // 1. 获取源镜像
    var sourceImage Image
    if err := m.store.Get(sourceImageID, &sourceImage); err != nil {
        return fmt.Errorf("source image not found: %w", err)
    }

    // 2. 创建新镜像（使用相同的内容但不同的名称和标签）
    newImage := &Image{
        ID:      generateImageID(targetName, targetTag),
        Name:    targetName,
        Tag:     targetTag,
        Layers:  sourceImage.Layers,
        Config:  sourceImage.Config,
        Size:    sourceImage.Size,
        Created: time.Now().Format(time.RFC3339),
    }

    // 3. 保存新镜像
    if err := m.store.Save(newImage.ID, newImage); err != nil {
        return fmt.Errorf("failed to save tagged image: %w", err)
    }

    logrus.Infof("Tagged image %s as %s:%s", sourceImageID, targetName, targetTag)
    return nil
}
```

**练习2.4.2**：实现镜像搜索功能
```go
func (m *Manager) SearchImages(query string) ([]*Image, error) {
    // 1. 获取所有镜像
    images, err := m.ListImages()
    if err != nil {
        return nil, err
    }

    // 2. 过滤匹配的镜像
    var results []*Image
    queryLower := strings.ToLower(query)

    for _, image := range images {
        // 在名称、标签中搜索
        if strings.Contains(strings.ToLower(image.Name), queryLower) ||
           strings.Contains(strings.ToLower(image.Tag), queryLower) {
            results = append(results, image)
        }
    }

    return results, nil
}
```

---

## 📚 示例3：理解容器管理

### 3.1 阅读目标
理解容器生命周期管理和状态转换。

### 3.2 关键文件
- `pkg/container/manager.go`

### 3.3 代码示例分析

#### 示例3.1：理解容器管理器结构

```go
// pkg/container/manager.go
type Manager struct {
    store      *store.Store    // 存储系统
    imageMgr   *image.Manager  // 镜像管理器
    containers map[string]*Container // 运行时容器缓存
    mu         sync.RWMutex     // 读写锁
}

func NewManager(store *store.Store, imageMgr *image.Manager) *Manager {
    return &Manager{
        store:      store,
        imageMgr:   imageMgr,
        containers: make(map[string]*Container),
    }
}
```

#### 示例3.2：理解运行容器逻辑

```go
func (m *Manager) RunContainer(imageName string, config *ContainerConfig) (*Container, error) {
    // 1. 验证镜像存在
    image, err := m.imageMgr.GetImage(imageName)
    if err != nil {
        return nil, fmt.Errorf("image not found: %w", err)
    }

    // 2. 创建容器对象
    container := &Container{
        ID:        generateContainerID(),
        Status:    StatusCreated,
        CreatedAt: time.Now().Format(time.RFC3339),
        Config:    *config,
    }

    // 3. 保存容器
    if err := m.store.Save(container.ID, container); err != nil {
        return nil, fmt.Errorf("failed to save container: %w", err)
    }

    // 4. 启动容器（异步）
    go m.startContainer(container)

    logrus.Infof("Created and started container %s based on image %s", container.ID, imageName)
    return container, nil
}

func (m *Manager) startContainer(container *Container) {
    // 1. 更新状态为运行中
    m.mu.Lock()
    container.Status = StatusRunning
    container.StartedAt = time.Now().Format(time.RFC3339)
    m.store.Save(container.ID, container)
    m.containers[container.ID] = container
    m.mu.Unlock()

    // 2. 模拟容器运行
    logrus.Infof("Container %s is running", container.ID)

    // 3. 监控容器状态
    go m.monitorContainer(container)
}
```

#### 示例3.3：理解停止容器逻辑

```go
func (m *Manager) StopContainer(containerID string) error {
    // 1. 获取容器
    m.mu.RLock()
    container, exists := m.containers[containerID]
    m.mu.RUnlock()

    if !exists {
        // 尝试从存储中加载
        var storedContainer Container
        if err := m.store.Get(containerID, &storedContainer); err != nil {
            return fmt.Errorf("container not found: %w", err)
        }
        container = &storedContainer
    }

    // 2. 检查容器状态
    if container.Status != StatusRunning {
        return fmt.Errorf("container is not running")
    }

    // 3. 停止容器进程
    if err := m.stopContainerProcess(containerID); err != nil {
        return fmt.Errorf("failed to stop container process: %w", err)
    }

    // 4. 更新状态
    m.mu.Lock()
    defer m.mu.Unlock()

    container.Status = StatusStopped
    container.FinishedAt = time.Now().Format(time.RFC3339)

    if err := m.store.Save(container.ID, container); err != nil {
        return fmt.Errorf("failed to save container state: %w", err)
    }

    delete(m.containers, containerID)

    logrus.Infof("Container %s stopped", containerID)
    return nil
}
```

### 3.4 实战练习

**练习3.4.1**：实现容器日志功能
```go
func (m *Manager) GetContainerLogs(containerID string) (string, error) {
    // 1. 获取容器
    var container Container
    if err := m.store.Get(containerID, &container); err != nil {
        return "", fmt.Errorf("container not found: %w", err)
    }

    // 2. 构建日志文件路径
    logPath := filepath.Join("/var/log/mydocker", containerID+".log")

    // 3. 读取日志文件
    logs, err := os.ReadFile(logPath)
    if err != nil {
        if os.IsNotExist(err) {
            return "", fmt.Errorf("no logs found for container %s", containerID)
        }
        return "", fmt.Errorf("failed to read logs: %w", err)
    }

    return string(logs), nil
}
```

**练习3.4.2**：实现容器统计信息
```go
func (m *Manager) GetContainerStats(containerID string) (*ContainerStats, error) {
    // 1. 获取容器
    var container Container
    if err := m.store.Get(containerID, &container); err != nil {
        return nil, fmt.Errorf("container not found: %w", err)
    }

    // 2. 检查容器状态
    if container.Status != StatusRunning {
        return nil, fmt.Errorf("container is not running")
    }

    // 3. 收集统计信息（模拟）
    stats := &ContainerStats{
        CPUUsage:    rand.Float64() * 100, // 随机CPU使用率
        MemoryUsage: rand.Int63n(512 * 1024 * 1024), // 随机内存使用
        NetworkIO: NetworkIO{
            BytesReceived: rand.Int63n(1024 * 1024),
            BytesSent:     rand.Int63n(1024 * 1024),
        },
        BlockIO: BlockIO{
            BytesRead:  rand.Int63n(1024 * 1024),
            BytesWritten: rand.Int63n(1024 * 1024),
        },
        Timestamp: time.Now().Format(time.RFC3339),
    }

    return stats, nil
}

type ContainerStats struct {
    CPUUsage    float64   `json:"cpu_usage"`
    MemoryUsage int64     `json:"memory_usage"`
    NetworkIO   NetworkIO `json:"network_io"`
    BlockIO     BlockIO   `json:"block_io"`
    Timestamp   string    `json:"timestamp"`
}

type NetworkIO struct {
    BytesReceived int64 `json:"bytes_received"`
    BytesSent     int64 `json:"bytes_sent"`
}

type BlockIO struct {
    BytesRead  int64 `json:"bytes_read"`
    BytesWritten int64 `json:"bytes_written"`
}
```

---

## 📚 示例4：理解CLI命令处理

### 4.1 阅读目标
理解CLI命令的解析和处理流程。

### 4.2 关键文件
- `pkg/cli/commands.go`

### 4.3 代码示例分析

#### 示例4.1：理解CLI应用结构

```go
// pkg/cli/commands.go
type App struct {
    cliApp       *cli.App
    store        *store.Store
    imageMgr     *image.Manager
    containerMgr *container.Manager
}

func New() (*App, error) {
    // 1. 创建存储系统
    store, err := store.NewStore("/var/lib/mydocker")
    if err != nil {
        return nil, fmt.Errorf("failed to create store: %v", err)
    }

    // 2. 创建镜像管理器
    imageMgr := image.NewManager(store)

    // 3. 创建容器管理器
    containerMgr := container.NewManager(store, imageMgr)

    // 4. 创建应用实例
    app := &App{
        store:        store,
        imageMgr:     imageMgr,
        containerMgr: containerMgr,
    }

    // 5. 配置CLI应用
    app.cliApp = &cli.App{
        Name:    "mydocker",
        Usage:   "A simple Docker implementation",
        Version: "1.0.0",
        Commands: []*cli.Command{
            app.createImageCommands(),
            app.createContainerCommands(),
            app.createSystemCommands(),
        },
    }

    return app, nil
}
```

#### 示例4.2：理解命令创建

```go
func (app *App) createImageCommands() *cli.Command {
    return &cli.Command{
        Name:  "image",
        Usage: "Manage images",
        Subcommands: []*cli.Command{
            {
                Name:    "pull",
                Usage:   "Pull an image from a registry",
                Aliases: []string{"p"},
                Flags: []cli.Flag{
                    &cli.StringFlag{
                        Name:  "tag",
                        Usage: "Image tag",
                        Value: "latest",
                    },
                },
                Action: app.pullImage,
            },
            {
                Name:    "list",
                Usage:   "List images",
                Aliases: []string{"ls"},
                Action:  app.listImages,
            },
            {
                Name:    "remove",
                Usage:   "Remove an image",
                Aliases: []string{"rm"},
                Action:  app.removeImage,
            },
        },
    }
}
```

#### 示例4.3：理解命令处理函数

```go
func (app *App) listImages(c *cli.Context) error {
    // 1. 调用业务逻辑
    images, err := app.imageMgr.ListImages()
    if err != nil {
        return fmt.Errorf("failed to list images: %v", err)
    }

    // 2. 格式化输出（表格形式）
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tSIZE\tCREATED")

    for _, image := range images {
        createdTime, _ := time.Parse(time.RFC3339, image.Created)
        timeAgo := time.Since(createdTime).Round(time.Hour)

        fmt.Fprintf(w, "%s\t%s\t%s\t%.2f MB\t%s ago\n",
            image.Name,
            image.Tag,
            image.ID[:12],
            float64(image.Size)/1024/1024,
            timeAgo,
        )
    }

    w.Flush()
    return nil
}
```

### 4.4 实战练习

**练习4.4.1**：添加version命令
```go
func (app *App) createSystemCommands() *cli.Command {
    return &cli.Command{
        Name:  "system",
        Usage: "Manage mydocker system",
        Subcommands: []*cli.Command{
            {
                Name:    "info",
                Usage:   "Display system-wide information",
                Action:  app.systemInfo,
            },
            {
                Name:    "prune",
                Usage:   "Remove unused data",
                Action:  app.systemPrune,
            },
            {
                Name:    "version",
                Usage:   "Show version information",
                Action:  app.systemVersion,
            },
        },
    }
}

func (app *App) systemVersion(c *cli.Context) error {
    versionInfo := struct {
        Version   string `json:"version"`
        GoVersion string `json:"go_version"`
        GitCommit string `json:"git_commit"`
        BuildTime string `json:"build_time"`
    }{
        Version:   "1.0.0",
        GoVersion: runtime.Version(),
        GitCommit: "unknown",
        BuildTime: time.Now().Format(time.RFC3339),
    }

    // 格式化输出
    fmt.Printf("MyDocker version %s\n", versionInfo.Version)
    fmt.Printf("Go version: %s\n", versionInfo.GoVersion)
    fmt.Printf("Git commit: %s\n", versionInfo.GitCommit)
    fmt.Printf("Built: %s\n", versionInfo.BuildTime)

    return nil
}
```

**练习4.4.2**：实现批量删除功能
```go
func (app *App) removeImage(c *cli.Context) error {
    if c.Args().Len() < 1 {
        return fmt.Errorf("please specify at least one image to remove")
    }

    force := c.Bool("force")
    var errors []error

    for _, imageRef := range c.Args().Slice() {
        // 解析镜像引用（name:tag格式）
        parts := strings.Split(imageRef, ":")
        var name, tag string
        if len(parts) == 1 {
            name = parts[0]
            tag = "latest"
        } else if len(parts) == 2 {
            name = parts[0]
            tag = parts[1]
        } else {
            errors = append(errors, fmt.Errorf("invalid image reference: %s", imageRef))
            continue
        }

        // 查找镜像
        images, err := app.imageMgr.ListImages()
        if err != nil {
            errors = append(errors, fmt.Errorf("failed to list images: %w", err))
            continue
        }

        var found bool
        for _, image := range images {
            if image.Name == name && image.Tag == tag {
                // 检查是否有容器使用此镜像
                if !force {
                    containers, err := app.containerMgr.ListContainers()
                    if err != nil {
                        errors = append(errors, fmt.Errorf("failed to check containers: %w", err))
                        continue
                    }

                    for _, container := range containers {
                        if container.ImageID == image.ID {
                            errors = append(errors, fmt.Errorf("image %s is being used by container %s", imageRef, container.ID[:12]))
                            continue
                        }
                    }
                }

                if err := app.imageMgr.RemoveImage(image.ID); err != nil {
                    errors = append(errors, fmt.Errorf("failed to remove image %s: %w", imageRef, err))
                } else {
                    fmt.Printf("Removed image: %s\n", imageRef)
                }
                found = true
                break
            }
        }

        if !found {
            errors = append(errors, fmt.Errorf("image not found: %s", imageRef))
        }
    }

    if len(errors) > 0 {
        fmt.Fprintf(os.Stderr, "Errors encountered:\n")
        for _, err := range errors {
            fmt.Fprintf(os.Stderr, "  - %v\n", err)
        }
        return fmt.Errorf("some images could not be removed")
    }

    return nil
}
```

---

## 📚 示例5：完整的端到端流程

### 5.1 阅读目标
理解从命令行到业务逻辑的完整执行流程。

### 5.2 完整示例：跟踪 `mydocker container run nginx`

```go
// 步骤1: main.go - 程序入口
func main() {
    app, err := cli.New()
    if err != nil {
        log.Fatal(err)
    }
    app.Run(os.Args) // ["mydocker", "container", "run", "nginx"]
}

// 步骤2: commands.go - 命令解析
func (app *App) createContainerCommands() *cli.Command {
    return &cli.Command{
        Name:  "container",
        Usage: "Manage containers",
        Subcommands: []*cli.Command{
            {
                Name:    "run",
                Usage:   "Run a command in a new container",
                Action:  app.runContainer,
            },
        },
    }
}

// 步骤3: commands.go - 命令处理
func (app *App) runContainer(c *cli.Context) error {
    if c.Args().Len() < 1 {
        return fmt.Errorf("please specify an image")
    }

    imageName := c.Args().First()
    command := c.Args().Tail()

    // 构建容器配置
    config := &container.ContainerConfig{
        Image:   imageName,
        Command: command,
    }

    // 调用容器管理器
    container, err := app.containerMgr.RunContainer(imageName, config)
    if err != nil {
        return fmt.Errorf("failed to run container: %v", err)
    }

    fmt.Printf("Container %s started based on image %s\n", container.ID[:12], imageName)
    return nil
}

// 步骤4: container/manager.go - 业务逻辑
func (m *Manager) RunContainer(imageName string, config *ContainerConfig) (*Container, error) {
    // 验证镜像存在
    image, err := m.imageMgr.GetImage(imageName)
    if err != nil {
        return nil, fmt.Errorf("image not found: %w", err)
    }

    // 创建容器
    container := &Container{
        ID:        generateContainerID(),
        Status:    StatusCreated,
        CreatedAt: time.Now().Format(time.RFC3339),
        Config:    *config,
    }

    // 保存容器
    if err := m.store.Save(container.ID, container); err != nil {
        return nil, fmt.Errorf("failed to save container: %w", err)
    }

    // 启动容器
    go m.startContainer(container)

    return container, nil
}

// 步骤5: container/manager.go - 启动容器
func (m *Manager) startContainer(container *Container) {
    // 更新状态
    m.mu.Lock()
    container.Status = StatusRunning
    container.StartedAt = time.Now().Format(time.RFC3339)
    m.store.Save(container.ID, container)
    m.containers[container.ID] = container
    m.mu.Unlock()

    // 模拟运行
    logrus.Infof("Container %s is running", container.ID)

    // 监控容器
    go m.monitorContainer(container)
}
```

### 5.3 实战练习：添加完整的日志系统

```go
// 1. 在容器配置中添加日志选项
type ContainerConfig struct {
    // ... 现有字段
    LogConfig LogConfig `json:"log_config"`
}

type LogConfig struct {
    Type      string `json:"type"`       // "json-file", "syslog", "none"
    MaxSize   string `json:"max_size"`   // "10m"
    MaxFiles  int    `json:"max_files"`  // 3
    Labels    map[string]string `json:"labels"`
}

// 2. 在容器管理器中实现日志处理
func (m *Manager) startContainerWithLogging(container *Container) {
    // 设置日志
    logger, err := m.setupContainerLogging(container)
    if err != nil {
        logrus.Errorf("Failed to setup logging for container %s: %v", container.ID, err)
    }

    // 启动容器
    go func() {
        defer logger.Close()

        // 重定向stdout/stderr到日志
        if logger != nil {
            stdoutPipe, err := logger.StdoutPipe()
            if err == nil {
                go io.Copy(stdoutPipe, os.Stdout)
            }
        }

        // 运行容器进程
        m.runContainerProcess(container)
    }()
}

// 3. 实现日志驱动
type LogDriver interface {
    Write(message string) error
    Read(since time.Time) ([]string, error)
    Close() error
}

type JSONFileLogger struct {
    filePath string
    file     *os.File
    mutex    sync.Mutex
}

func (l *JSONFileLogger) Write(message string) error {
    l.mutex.Lock()
    defer l.mutex.Unlock()

    logEntry := struct {
        Timestamp string `json:"timestamp"`
        Message   string `json:"message"`
    }{
        Timestamp: time.Now().Format(time.RFC3339Nano),
        Message:   message,
    }

    data, err := json.Marshal(logEntry)
    if err != nil {
        return err
    }

    _, err = l.file.Write(append(data, '\n'))
    return err
}
```

---

## 🎯 学习路径总结

### 基础阶段（1-2周）
1. **理解数据结构** - 学习Types模块
2. **掌握存储系统** - 学习Store模块
3. **理解镜像管理** - 学习Image模块
4. **理解容器管理** - 学习Container模块
5. **掌握CLI处理** - 学习CLI模块

### 进阶阶段（2-3周）
1. **运行完整流程** - 跟踪命令执行
2. **扩展基础功能** - 添加新命令和功能
3. **理解测试框架** - 学习如何编写测试
4. **实践项目** - 实现小型功能

### 高级阶段（根据兴趣）
1. **性能优化** - 学习Performance模块
2. **网络功能** - 学习Network模块
3. **存储驱动** - 学习Storage模块
4. **集群管理** - 学习Cluster模块

## 💡 学习建议

1. **多动手实践**：不要只看不练，每个例子都要亲自运行
2. **逐步深入**：先理解简单概念，再学习复杂功能
3. **多做练习**：通过修改和扩展代码来加深理解
4. **查看测试**：测试代码是理解功能如何工作的最佳途径
5. **记录笔记**：记录学习过程中的问题和解决方案

记住，学习源码是一个循序渐进的过程，不要急于求成。通过这些示例和练习，你将能够深入理解Docker的实现原理！