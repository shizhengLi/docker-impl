# MyDocker - 简化版Docker实现

这是一个简化版的Docker容器运行时实现，包含Docker的核心功能。

## 项目结构

```
docker-impl/
├── cmd/                    # CLI入口
│   └── mydocker/         # 主程序
├── pkg/                   # 核心包
│   ├── cli/              # CLI命令实现
│   ├── container/        # 容器管理
│   ├── image/            # 镜像管理
│   ├── store/            # 存储管理
│   └── types/            # 数据类型定义
├── tests/                 # 测试
│   └── integration/      # 集成测试
├── DESIGN.md             # 设计文档
├── go.mod               # Go模块
└── README.md            # 项目说明
```

## 核心功能

### 镜像管理
- `mydocker image pull <image>` - 拉取镜像
- `mydocker image list` - 列出本地镜像
- `mydocker image remove <image>` - 删除镜像
- `mydocker image build` - 构建镜像
- `mydocker image tag` - 镜像标签管理

### 容器管理
- `mydocker container run <image> <command>` - 运行容器
- `mydocker container list` - 列出容器
- `mydocker container start <container>` - 启动容器
- `mydocker container stop <container>` - 停止容器
- `mydocker container remove <container>` - 删除容器
- `mydocker container logs <container>` - 查看日志
- `mydocker container inspect <container>` - 检查容器

### 系统管理
- `mydocker system info` - 系统信息
- `mydocker system prune` - 清理未使用的数据

## 架构设计

### 核心模块
1. **CLI模块** - 命令行接口
2. **Daemon模块** - 主进程管理
3. **镜像管理模块** - 镜像存储和管理
4. **容器管理模块** - 容器生命周期管理
5. **存储模块** - 分层文件系统
6. **网络模块** - 网络配置

### 隔离机制
- **命名空间隔离** - PID、网络、挂载、UTS、IPC
- **资源限制** - cgroups实现CPU、内存限制
- **文件系统** - Union File System

## 技术栈

- **语言**: Go 1.21+
- **容器技术**: Linux namespaces, cgroups
- **存储**: Union File System (模拟)
- **网络**: 网桥模式
- **测试**: Go testing framework + testify

## 构建和运行

### 前提条件
- Go 1.21+
- Linux环境（支持namespaces和cgroups）

### 构建
```bash
go build -o mydocker ./cmd/mydocker
```

### 运行测试
```bash
# 单元测试
go test ./pkg/...

# 集成测试
go test ./tests/integration/...
```

### 使用示例
```bash
# 拉取镜像
./mydocker image pull alpine

# 列出镜像
./mydocker image list

# 运行容器
./mydocker container run alpine echo "Hello World"

# 列出容器
./mydocker container list --all

# 查看容器日志
./mydocker container logs <container_id>

# 停止容器
./mydocker container stop <container_id>

# 删除容器
./mydocker container remove <container_id>
```

## 设计原则

1. **小步快跑** - 分阶段实现，每阶段都有完整的测试
2. **简单可靠** - 保持代码简洁，易于理解和维护
3. **安全第一** - 实现基本的隔离和安全机制
4. **可扩展性** - 模块化设计，便于功能扩展

## 测试策略

### 单元测试
- 每个模块的独立功能测试
- 边界条件和错误处理测试
- 使用testify进行断言

### 集成测试
- 端到端功能测试
- 完整的工作流程测试
- 错误恢复和并发测试

### 性能测试
- 基准测试
- 并发性能测试
- 资源使用测试

## 开发状态

- [x] 需求分析和设计
- [x] 核心功能实现
- [x] 单元测试
- [x] 集成测试
- [ ] 性能优化
- [ ] 更多网络功能
- [ ] 存储驱动优化
- [ ] 集群支持

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 编写测试
4. 提交变更
5. 发起Pull Request

## 许可证

MIT License

## 注意事项

这是一个学习和演示项目，仅用于理解Docker的核心原理。在生产环境中，请使用官方的Docker产品。