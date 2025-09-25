# 🚀 MyDocker 快速开始指南

## 📋 快速概览

这是一个完整的Docker容器运行时实现，包含所有核心功能和高级特性。本指南帮助你快速了解项目结构和基本使用。

## 🏗️ 项目结构

```
docker-impl/
├── cmd/mydocker/          # 程序入口点
├── pkg/                   # 核心功能实现
│   ├── types/            # 数据类型定义（👈从这里开始）
│   ├── store/            # 存储系统
│   ├── image/            # 镜像管理
│   ├── container/        # 容器管理
│   ├── cli/              # 命令行接口
│   ├── performance/      # 性能优化
│   ├── network/          # 网络功能
│   ├── storage/          # 存储驱动
│   └── cluster/          # 集群管理
├── docs/                 # 📚 学习文档
└── tests/                # 测试代码
```

## 🛠️ 环境准备

### 系统要求
- Go 1.21+
- Linux 系统（推荐 Ubuntu 20.04+）
- 基本的命令行操作能力

### 快速检查
```bash
# 检查Go版本
go version

# 检查系统（应该显示Linux）
uname

# 进入项目目录
cd docker-impl
```

## 🚀 5分钟快速体验

### 1. 编译项目
```bash
# 编译主程序
go build -o mydocker ./cmd/mydocker

# 检查编译结果
./mydocker --version
```

### 2. 基本功能测试
```bash
# 查看帮助
./mydocker --help

# 测试镜像功能
./mydocker image list
./mydocker image pull test-image

# 测试容器功能
./mydocker container run test-image echo "Hello World"
./mydocker container list

# 测试系统功能
./mydocker system info
```

### 3. 集群功能测试（高级）
```bash
# 初始化集群
./mydocker cluster init --advertise-addr 192.168.1.100

# 查看集群状态
./mydocker cluster status

# 列出节点
./mydocker node ls
```

## 📖 学习路径推荐

### 🌱 完全初学者（3-4周）

**第1周：理解基础**
- 阅读文档：[源码阅读指南](./SOURCE_CODE_GUIDE.md)
- 查看代码：`pkg/types/`
- 运行测试：`go test ./pkg/store`

**第2周：核心功能**
- 阅读文档：[核心概念详解](./CORE_CONCEPTS.md)
- 查看代码：`pkg/store/`, `pkg/image/`
- 运行测试：`go test ./pkg/image`

**第3周：业务逻辑**
- 阅读文档：[模块依赖关系](./MODULE_DEPENDENCIES.md)
- 查看代码：`pkg/container/`, `pkg/cli/`
- 运行测试：`go test ./pkg/container`

**第4周：实践练习**
- 阅读文档：[代码示例练习](./CODE_READING_EXAMPLES.md)
- 完成练习，修改代码，添加功能

### 🚀 有经验的Go开发者（1-2周）

**第1天：项目概览**
- 快速浏览所有文档
- 运行所有测试
- 理解项目结构

**第2-3天：核心模块**
- 深入阅读`pkg/types/`和`pkg/store/`
- 理解数据结构和存储机制
- 完成基础练习

**第4-7天：业务逻辑**
- 阅读`pkg/image/`和`pkg/container/`
- 理解业务逻辑和API设计
- 完成进阶练习

**第8-14天：高级特性**
- 研究高级功能模块
- 理解架构设计
- 尝试添加新功能

## 🧪 运行测试

### 单元测试
```bash
# 运行所有单元测试
go test ./pkg/...

# 运行特定模块测试
go test ./pkg/store
go test ./pkg/image
go test ./pkg/container

# 查看测试覆盖率
go test -cover ./pkg/...
```

### 集成测试
```bash
# 运行集成测试
go test ./tests/integration/...

# 运行带详细输出的测试
go test -v ./tests/integration/
```

## 🔍 代码阅读技巧

### 1. 从数据类型开始
```go
// pkg/types/image.go
type Image struct {
    ID       string      `json:"id"`
    Name     string      `json:"name"`
    Tag      string      `json:"tag"`
    // ...
}

// pkg/types/container.go
type Container struct {
    ID         string           `json:"id"`
    Name       string           `json:"name"`
    Status     ContainerStatus  `json:"status"`
    // ...
}
```

### 2. 跟随一个命令的执行
```bash
# 命令：./mydocker image list
# 执行路径：
# 1. main.go -> 2. cli/commands.go -> 3. image/manager.go -> 4. store/store.go
```

### 3. 理解错误处理模式
```go
func doSomething() error {
    // 1. 验证输入
    if input == nil {
        return fmt.Errorf("input cannot be nil")
    }

    // 2. 调用其他函数
    result, err := someFunction()
    if err != nil {
        return fmt.Errorf("failed to do something: %w", err)
    }

    return nil
}
```

## 🎯 快速练习

### 练习1：添加简单的version命令
```go
// 在pkg/cli/commands.go中添加
func (app *App) systemVersion(c *cli.Context) error {
    fmt.Printf("MyDocker version 1.0.0\n")
    fmt.Printf("Go version: %s\n", runtime.Version())
    return nil
}
```

### 练习2：理解数据结构
```go
// 创建一个镜像实例
image := &types.Image{
    ID:   "test-image-123",
    Name: "nginx",
    Tag:  "latest",
    Size: 142000000,
}

fmt.Printf("镜像名称: %s:%s\n", image.Name, image.Tag)
fmt.Printf("镜像大小: %.2f MB\n", float64(image.Size)/1024/1024)
```

### 练习3：使用存储系统
```go
// 创建存储实例
store := store.NewStore("/tmp/test")

// 保存数据
err := store.Save("test-key", map[string]string{"hello": "world"})

// 读取数据
var result map[string]string
err = store.Get("test-key", &result)

fmt.Printf("读取结果: %v\n", result)
```

## 📚 扩展阅读

### 必读文档
1. [📖 源码阅读指南](./SOURCE_CODE_GUIDE.md) - 完整的学习路径
2. [💡 核心概念详解](./CORE_CONCEPTS.md) - 技术原理解析
3. [📊 模块依赖关系](./MODULE_DEPENDENCIES.md) - 代码结构详解
4. [💻 代码示例练习](./CODE_READING_EXAMPLES.md) - 实战练习项目

### 学习资源
- [Go语言官方教程](https://go.dev/tour/)
- [Docker概念介绍](https://docs.docker.com/get-started/overview/)
- [Linux容器技术](https://linuxcontainers.org/)

## 🆘 遇到问题？

### 常见问题
1. **编译失败**：检查Go版本是否为1.21+
2. **测试失败**：确保在Linux环境下运行
3. **权限问题**：确保有文件系统写入权限
4. **依赖问题**：运行`go mod tidy`

### 获取帮助
1. **查看文档**：先查阅相关文档
2. **查看测试**：测试代码展示了正确用法
3. **添加日志**：在代码中添加print语句
4. **社区交流**：与其他学习者讨论

---

**祝你学习愉快！🎉**

记住，学习是一个循序渐进的过程。按照推荐的学习路径，你将能够深入理解容器技术的精髓！