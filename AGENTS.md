# DBpackLogs 项目智能体上下文

## 项目概述

DBpackLogs 是一款企业级的数据库日志收集打包工具，通过 SSH 远程连接到数据库节点，自动完成以下任务：

- 自动识别数据库类型（Greenplum / PostgreSQL / openGauss）
- 自动发现集群中的所有节点
- 按时间范围收集数据库日志
- 收集数据库配置文件
- 收集操作系统诊断信息
- 生成收集报告
- 打包输出为 ZIP 或 TAR.GZ 格式

### 核心特性

- **多数据库支持**：自动识别 Greenplum、PostgreSQL、openGauss
- **自动节点发现**：自动发现集群中的所有节点（coordinator / primary / standby / segment）
- **时间范围过滤**：按指定时间范围精确收集日志，支持多种时间格式
- **配置文件收集**：自动收集 `postgresql.conf`、`pg_hba.conf`、`pg_ident.conf` 等
- **OS 信息收集**：收集 CPU、内存、磁盘、网络、系统日志等
- **智能打包**：支持 ZIP 和 TAR.GZ 格式输出
- **错误容忍**：单节点失败不影响其他节点收集

### 高级特性

- **SSH 连接池**：复用 SSH 连接，提升多节点收集效率
- **指数退避重试**：网络错误自动重试，避免瞬时故障
- **并发收集**：多节点并行收集（最大并发 10），缩短总耗时
- **信号处理**：支持 Ctrl+C / SIGTERM 优雅退出
- **调试模式**：`--verbose` 开启详细的 DEBUG 日志输出

## 项目架构

### 模块结构

```
dbpacklogs/
├── cmd/
│   └── main.go                  # 命令行入口
├── internal/
│   ├── collector/
│   │   ├── orchestrator.go      # 收集流程编排器
│   │   ├── db_collector.go      # DB 配置和日志收集
│   │   └── os_collector.go      # OS 诊断信息收集
│   ├── config/
│   │   └── config.go            # 配置结构、校验、/etc/hosts 解析
│   ├── detector/
│   │   ├── adapter.go           # DBAdapter 接口 + NodeInfo
│   │   ├── factory.go           # 自动探测工厂（SELECT version()）
│   │   ├── greenplum.go         # Greenplum 适配器
│   │   ├── postgres.go          # PostgreSQL 适配器
│   │   └── opengauss.go         # openGauss 适配器（SSH + cm_ctl/gs_om）
│   ├── filter/
│   │   └── time_filter.go       # 时间范围过滤、dmesg 时间戳解析
│   ├── packager/
│   │   ├── packager.go          # 打包器接口
│   │   ├── zip_packager.go      # ZIP 实现
│   │   ├── tar_packager.go      # TAR.GZ 实现
│   │   └── organizer.go         # 工作目录创建和目录布局
│   ├── report/
│   │   └── report.go            # 文本报告 + JSON 元数据
│   └── ssh/
│       ├── client.go            # SSH 客户端、认证、重试
│       ├── pool.go              # SSH 连接池
│       └── transfer.go          # SFTP 下载、远端 tar 流、远端 find
└── pkg/
    └── utils/
        ├── logger.go            # Zap logger 封装
        └── format.go            # 时间解析、字节/耗时格式化
```

### 核心模块说明

| 模块 | 职责 |
|------|------|
| `cmd` | 命令行入口，参数解析 |
| `collector` | 日志收集全流程编排 |
| `detector` | 数据库类型识别和节点发现 |
| `filter` | 时间范围过滤，dmesg 时间戳解析 |
| `packager` | ZIP / TAR.GZ 打包和目录组织 |
| `report` | 文本报告和 JSON 元数据生成 |
| `ssh` | SSH 客户端、连接池、SFTP 传输、远端 tar 流 |
| `config` | 配置结构、参数校验、`/etc/hosts` 解析 |
| `utils` | Logger、时间解析、字节/耗时格式化 |

## 构建和运行

### 构建命令

```bash
make build           # 构建当前平台 → ./bin/dbpacklogs
make build-linux-amd64
make build-linux-arm64
make test            # 运行测试
make lint            # 运行代码检查
make clean           # 清理构建产物
```

### 项目依赖

- Go 1.24
- github.com/jackc/pgx/v5
- github.com/pkg/sftp
- github.com/spf13/cobra
- go.uber.org/zap
- golang.org/x/crypto
- golang.org/x/sync

## 开发约定

### 代码风格

- 使用 Go 1.24 最新特性
- 遵循 Go 语言标准编码规范
- 使用 zap 作为日志库
- 使用 cobra 作为 CLI 框架
- 使用 errgroup + semaphore 控制并发（最大 10 并发）

### 测试实践

- 使用表驱动测试
- 测试成功和错误路径
- 包含边界条件和边缘情况
- 测试并发访问
- 避免依赖外部服务的测试

### 项目特定注意事项

- 最大并发节点数：10
- 最大时间范围：3 天（自动截断）
- 数据库支持：Greenplum、PostgreSQL、openGauss
- SSH 命令构造必须防止注入
- 文件路径验证（isValidRemotePath）
- 临时目录的正确清理

## 智能体使用指南

### 1. Refactoring Agent (refactor)
**用途**: 代码重构和结构优化

**职责**:
- 提取重复代码到辅助函数
- 简化复杂条件逻辑
- 减少函数长度和复杂度
- 改进命名清晰度
- 移除死代码
- 合并重复逻辑

### 2. Documentation Updater (doc-updater)
**用途**: 维护和更新项目文档

**职责**:
- 同步更新 README.md (英文) 和 README_CN.md (中文)
- 确保两个文件顶部互相链接
- 记录新的 CLI 参数和功能
- 更新使用示例
- 记录破坏性变更

### 3. Test Writer (test-writer)
**用途**: 编写全面的单元测试

**职责**:
- 编写 Go 单元测试
- 使用表驱动测试
- 测试成功和错误路径
- 包含边界条件和边缘情况
- 测试并发访问

### 4. Code Reviewer (code-reviewer)
**用途**: 代码质量和安全审查

**职责**:
- 检查命令注入漏洞（特别是 SSH 命令）
- 验证所有用户输入和文件路径
- 确保正确的错误处理
- 检查硬编码凭证或密钥
- 遵循 Go 最佳实践

## 项目特定上下文

所有 agents 都应了解以下项目信息：
- Go 1.24 项目
- 数据库支持: Greenplum / PostgreSQL / openGauss
- SSH 远程操作
- 最大并发节点数: 10
- 最大时间范围: 3 天
- 关键包: detector, collector, packager, report, filter, config