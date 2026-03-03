# DBpackLogs - 数据库日志收集打包工具

[English](./README_EN.md) | 中文

## 目录

1. [项目介绍](#1-项目介绍)
2. [功能特性](#2-功能特性)
3. [系统要求](#3-系统要求)
4. [支持的数据类型](#4-支持的数据类型)
5. [安装指南](#5-安装指南)
6. [快速开始](#6-快速开始)
7. [命令参数详解](#7-命令参数详解)
8. [使用示例](#8-使用示例)
9. [输出说明](#9-输出说明)
10. [工作原理](#10-工作原理)
11. [架构设计](#11-架构设计)
12. [配置示例](#12-配置示例)
13. [故障排查](#13-故障排查)
14. [常见问题](#14-常见问题)
15. [开发指南](#15-开发指南)

---

## 1. 项目介绍

DBpackLogs 是一款企业级的数据库日志收集打包工具，通过 SSH 远程连接到数据库节点，自动完成以下任务：

- 自动识别数据库类型（Greenplum/PostgreSQL/openGauss）
- 自动发现集群中的所有节点
- 按时间范围收集数据库日志
- 收集数据库配置文件
- 收集操作系统诊断信息
- 生成收集报告
- 打包输出为 ZIP 或 TAR.GZ 格式

### 适用场景

- 数据库故障排查和诊断
- 数据库性能分析
- 数据库审计和合规
- 数据库升级迁移前的数据收集
- 日常数据库巡检

---

## 2. 功能特性

### 2.1 核心功能

| 功能 | 说明 |
|------|------|
| **多数据库支持** | 自动识别 Greenplum、PostgreSQL、openGauss |
| **自动节点发现** | 自动发现集群中的所有节点（master/coordinator/primary/standby/segment） |
| **时间范围过滤** | 按指定时间范围精确收集日志，支持多种时间格式 |
| **配置文件收集** | 自动收集 postgresql.conf、pg_hba.conf、pg_ident.conf 等 |
| **OS 信息收集** | 收集 CPU、内存、磁盘、网络、系统日志等 |
| **智能打包** | 支持 ZIP 和 TAR.GZ 格式输出 |
| **错误容忍** | 单节点失败不影响其他节点收集 |

### 2.2 高级特性

| 特性 | 说明 |
|------|------|
| **SSH 连接池** | 复用 SSH 连接，提升多节点收集效率 |
| **指数退避重试** | 网络错误自动重试，避免瞬时故障 |
| **并发收集** | 多节点并行收集，缩短总耗时 |
| **信号处理** | 支持 Ctrl+C 优雅退出 |
| **调试模式** | 详细的调试日志输出 |

---

## 3. 系统要求

### 3.1 运行要求

| 要求 | 说明 |
|------|------|
| **操作系统** | Linux (CentOS、RHEL、Ubuntu、Debian 等) |
| **Go 版本** | Go 1.21 或更高版本 |
| **SSH 访问** | 能够访问数据库节点的 SSH |
| **磁盘空间** | 根据日志量预留足够空间 |

### 3.2 目标节点要求

| 组件 | 要求 |
|------|------|
| **SSH 服务** | 目标节点需要开启 SSH 服务 |
| **数据库** | Greenplum 5.x+ / PostgreSQL 9.x+ / openGauss 3.x+ |
| **磁盘空间** | 日志目录所在分区需要有足够空间 |

---

## 4. 支持的数据类型

### 4.1 Greenplum

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 5.x | ✅ 支持 | gp_segment_configuration |
| 6.x | ✅ 支持 | gp_segment_configuration |
| 7.x | ✅ 支持 | gp_segment_configuration |

**节点角色**：
- coordinator：协调节点
- primary：主节点
- mirror：镜像节点
- standby：备节点
- segment：数据分片节点

### 4.2 PostgreSQL

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 9.x | ✅ 支持 | pg_stat_replication |
| 10.x | ✅ 支持 | pg_stat_replication |
| 11.x - 16.x | ✅ 支持 | pg_stat_replication |

**节点角色**：
- primary：主节点
- standby：备节点

**复制方式**：
- 流复制 (Streaming Replication)
- 逻辑复制 (Logical Replication)
- 物理复制 (Physical Replication)

### 4.3 openGauss

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 3.0.x | ✅ 支持 | cm_ctl / gs_om |
| 3.1.x | ✅ 支持 | cm_ctl / gs_om |
| 5.0.x | ✅ 支持 | cm_ctl / gs_om |

**节点角色**：
- primary：主节点
- standby：备节点
- cascade standby：级联备节点

---

## 5. 安装指南

### 5.1 从源码构建

```bash
# 克隆项目
git clone https://github.com/your-repo/dbpacklogs.git
cd dbpacklogs

# 构建
make build

# 验证
./bin/dbpacklogs --help
```

### 5.2 交叉编译

```bash
# Linux AMD64
make build-linux-amd64

# Linux ARM64
make build-linux-arm64

# macOS AMD64
make build-darwin-amd64

# macOS ARM64
make build-darwin-arm64
```

### 5.3 预编译版本

从 [Releases](https://github.com/your-repo/dbpacklogs/releases) 页面下载对应平台的二进制文件。

---

## 6. 快速开始

### 6.1 最简单的使用

```bash
# 单节点收集
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin

# 收集最近3天的日志
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --start-time '2026-02-20' --end-time '2026-02-27'
```

### 6.2 完整示例

```bash
./dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11,10.0.0.12 \
  --ssh-user gpadmin \
  --ssh-password 'YourPassword' \
  --db-user gpadmin \
  --db-password 'DbPassword' \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/backup \
  --verbose
```

---

## 7. 命令参数详解

### 7.1 必填参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--ssh-user` | SSH 用户名 | `--ssh-user gpadmin` |

### 7.2 节点参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--hosts` | - | 指定节点列表，逗号分隔 |
| `--all-hosts` | false | 从 /etc/hosts 读取所有节点 |

### 7.3 SSH 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--ssh-port` | 22 | SSH 端口 |
| `--ssh-password` | - | SSH 密码 |

### 7.4 数据库参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--db-port` | 5432 | 数据库端口 |
| `--db-user` | postgres | 数据库用户名 |
| `--db-password` | - | 数据库密码 |
| `--db-name` | postgres | 数据库名称 |

### 7.5 时间参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--start-time` | 最近3天 | 收集起始时间 |
| `--end-time` | 当前时间 | 收集结束时间 |

**时间格式**：
- `2006-01-02 15:04:05` - 标准格式
- `2006-01-02T15:04:05` - ISO 格式
- `2006-01-02` - 日期格式
- `20060102` - 紧凑格式

### 7.6 输出参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--output` | . | 输出目录 |
| `--pack-type` | zip | 打包类型 (zip/tar) |
| `--verbose` | false | 启用调试日志 |

---

## 8. 使用示例

### 8.1 基础使用

```bash
# 1. 查看帮助
./dbpacklogs --help

# 2. 收集单节点日志（默认最近3天）
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin

# 3. 收集多节点日志
./dbpacklogs --hosts 10.0.0.10,10.0.0.11,10.0.0.12 --ssh-user gpadmin

# 4. 使用 all-hosts 模式
./dbpacklogs --all-hosts --ssh-user gpadmin

# 5. 指定输出目录
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --output /data/backup
```

### 8.2 时间范围过滤

```bash
# 6. 指定日期范围
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --start-time '2026-02-20' --end-time '2026-02-25'

# 7. 收集单日日志
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --start-time '2026-02-24' --end-time '2026-02-25'

# 8. 带时分秒的时间范围
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --start-time '2026-02-24 08:00:00' --end-time '2026-02-24 18:00:00'

# 9. ISO 格式时间
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --start-time '2026-02-24T00:00:00' --end-time '2026-02-25T00:00:00'
```

### 8.3 认证方式

```bash
# 10. 使用 SSH 密码
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-password 'P@ssw0rd'

# 11. 使用默认 SSH 密钥
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin

# 12. 自定义 SSH 端口
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-port 2222

# 13. 指定数据库密码
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --db-password 'DbP@ss'
```

### 8.4 打包格式

```bash
# 14. ZIP 格式（默认）
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --pack-type zip

# 15. TAR.GZ 格式
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --pack-type tar
```

### 8.5 调试模式

```bash
# 16. 启用调试日志
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --verbose
```

### 8.6 数据库特定

```bash
# 17. Greenplum 集群
./dbpacklogs --all-hosts --ssh-user gpadmin --db-port 5432 --db-user gpadmin --db-name gpadmin

# 18. PostgreSQL 主从
./dbpacklogs --hosts 10.0.0.10,10.0.0.11 --ssh-user postgres --db-port 5432 --db-user postgres

# 19. openGauss 集群
./dbpacklogs --all-hosts --ssh-user omm --db-port 5432 --db-user gaussdb
```

### 8.7 组合场景

```bash
# 20. 完整参数示例
./dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user gpadmin \
  --ssh-password 'P@ssw0rd' \
  --db-port 5432 \
  --db-user gpadmin \
  --db-password 'DbP@ss' \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/backup \
  --verbose
```

---

## 9. 输出说明

### 9.1 输出目录结构

```
/data/backup/
└── DBpackLogs_20260227_143000/
    ├── greenplum/                    # 按数据库类型组织
    │   ├── 10.0.0.10/               # 按节点 IP 组织
    │   │   ├── db_info/             # 数据库配置信息
    │   │   │   ├── postgresql.conf
    │   │   │   ├── pg_hba.conf
    │   │   │   ├── pg_ident.conf
    │   │   │   └── cluster_topology.txt
    │   │   ├── db_logs/             # 数据库日志
    │   │   │   └── pg_log_10.0.0.10_pg_log.tar.gz
    │   │   └── os_info/             # 操作系统信息
    │   │       ├── cpu.txt
    │   │       ├── memory.txt
    │   │       ├── disk.txt
    │   │       ├── network.txt
    │   │       ├── dmesg.txt
    │   │       ├── journalctl.txt
    │   │       └── ...
    │   └── 10.0.0.11/
    │       └── ...
    ├── collection_report.txt         # 收集报告（文本）
    └── metadata.json                 # 元数据（JSON）
```

### 9.2 收集报告示例

```
=== DBpackLogs 收集报告 ===
生成时间  : 2026-02-27 14:30:00
数据库类型: greenplum
时间范围  : 2026-02-20 00:00:00 ~ 2026-02-27 14:30:00
总节点数  : 3
成功节点  : 3
失败节点  : 0
总耗时    : 2m30s

--- 成功节点 ---
  [OK] 10.0.0.10       role=coordinator  elapsed=45.2s
  [OK] 10.0.0.11       role=primary     elapsed=42.1s
  [OK] 10.0.0.12       role=standby     elapsed=38.5s
```

### 9.3 元数据 JSON 示例

```json
{
  "db_type": "greenplum",
  "total_nodes": 3,
  "success_nodes": 3,
  "failed_nodes": 0,
  "start_time": "2026-02-20 00:00:00",
  "end_time": "2026-02-27 14:30:00",
  "generated_at": "2026-02-27 14:30:00",
  "total_duration": "2m30s",
  "nodes": [
    {
      "host": "10.0.0.10",
      "role": "coordinator",
      "success": true,
      "elapsed_ms": 45200
    }
  ]
}
```

---

## 10. 工作原理

### 10.1 执行流程

```
┌─────────────────────────────────────────────────────────────────┐
│                      DBpackLogs 执行流程                         │
└─────────────────────────────────────────────────────────────────┘

1. 初始化
   ├── 解析命令行参数
   ├── 初始化日志系统
   ├── 验证参数合法性
   ├── 创建工作目录
   └── 初始化 SSH 连接池

2. 数据库探测
   ├── 通过 SSH 连接到第一个节点
   ├── 使用 pgx 连接数据库
   ├── 执行 SELECT version()
   └── 根据返回结果识别数据库类型

3. 节点发现
   ├── Greenplum: gp_segment_configuration
   ├── PostgreSQL: pg_stat_replication + pg_catalog
   └── openGauss: cm_ctl / gs_om

4. 日志收集（并发）
   ├── 对每个节点：
   │   ├── 获取 SSH 连接
   │   ├── 下载数据库配置文件
   │   ├── 收集数据库日志（按时间过滤）
   │   └── 收集 OS 诊断信息
   └── errgroup 并发控制

5. 生成报告
   ├── 生成文本报告
   └── 生成 JSON 元数据

6. 打包输出
   ├── 打包为 ZIP 或 TAR.GZ
   └── 清理临时文件
```

### 10.2 数据库类型识别

```go
// 通过 version() 返回值识别
switch {
case strings.Contains(versionLower, "greenplum"):
    // Greenplum
case strings.Contains(versionLower, "opengauss"), 
     strings.Contains(versionLower, "gaussdb"):
    // openGauss
case strings.Contains(versionLower, "postgresql"):
    // PostgreSQL
}
```

### 10.3 节点发现机制

**Greenplum**:
```sql
SELECT address, port, role, preferred_role, datadir
FROM gp_segment_configuration
WHERE status = 'u'
ORDER BY content, role
```

**PostgreSQL**:
```sql
-- 主节点
SELECT current_setting('data_directory')

-- Standby 节点
SELECT client_addr, application_name
FROM pg_stat_replication
WHERE state = 'streaming'
```

**openGauss**:
```bash
# 方式1: cm_ctl
cm_ctl query -Cv

# 方式2: gs_om
gs_om -t status --detail
```

---

## 11. 架构设计

### 11.1 模块架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         cmd/main.go                             │
│                      命令行入口层                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     collector/orchestrator                      │
│                       编排协调层                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ DBCollector  │  │ OSCollector  │  │   Reporter   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
          │                    │                    │
          ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                          detector                               │
│                         适配器层                                 │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                │
│  │ Greenplum  │  │  Postgres  │  │ OpenGauss  │                │
│  └────────────┘  └────────────┘  └────────────┘                │
└─────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────┐
│                            ssh                                  │
│                          传输层                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                │
│  │   Client   │  │    Pool    │  │  Transfer  │                │
│  └────────────┘  └────────────┘  └────────────┘                │
└─────────────────────────────────────────────────────────────────┘
```

### 11.2 核心模块说明

| 模块 | 职责 |
|------|------|
| `cmd` | 命令行入口，参数解析 |
| `collector` | 日志收集编排 |
| `detector` | 数据库类型识别和节点发现 |
| `filter` | 时间范围过滤 |
| `packager` | 打包输出 |
| `report` | 报告生成 |
| `ssh` | SSH 连接和文件传输 |
| `config` | 配置管理 |
| `utils` | 工具函数 |

### 11.3 并发模型

```go
// 使用 errgroup 实现并发收集
eg, egCtx := errgroup.WithContext(ctx)

for _, node := range nodes {
    eg.Go(func() error {
        // 检查上下文取消
        select {
        case <-egCtx.Done():
            return egCtx.Err()
        default:
        }
        
        // 收集节点日志
        // ...
        
        return nil
    })
}

eg.Wait()
```

---

## 12. 配置示例

### 12.1 Greenplum 集群

```bash
# 收集整个 Greenplum 集群
./dbpacklogs \
  --all-hosts \
  --ssh-user gpadmin \
  --ssh-password 'P@ssw0rd' \
  --db-user gpadmin \
  --db-password 'gpadmin123' \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/gp_backup
```

### 12.2 PostgreSQL 主从

```bash
# 收集 PostgreSQL 主从集群
./dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user postgres \
  --ssh-password 'P@ssw0rd' \
  --db-user postgres \
  --db-password 'postgres123' \
  --start-time '2026-02-24' \
  --end-time '2026-02-25' \
  --pack-type zip \
  --output /data/pg_backup
```

### 12.3 openGauss 集群

```bash
# 收集 openGauss 集群
./dbpacklogs \
  --all-hosts \
  --ssh-user omm \
  --ssh-password 'P@ssw0rd' \
  --db-user gaussdb \
  --db-password 'gaussdb123' \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/og_backup
```

---

## 13. 故障排查

### 13.1 常见错误

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `SSH connection refused` | SSH 服务未启动或端口错误 | 检查 SSH 服务和端口 |
| `Authentication failed` | 用户名或密码错误 | 验证认证信息 |
| `Database connection failed` | 数据库参数错误 | 检查 db-host、db-port、db-user |
| `Permission denied` | 权限不足 | 检查 SSH 用户权限 |
| `No such file or directory` | 输出目录不存在 | 使用 --output 指定存在的目录 |

### 13.2 调试模式

使用 `--verbose` 参数查看详细日志：

```bash
./dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --verbose
```

### 13.3 日志分析

收集完成后查看报告：

```bash
# 查看文本报告
cat DBpackLogs_20260227_143000/collection_report.txt

# 查看 JSON 元数据
cat DBpackLogs_20260227_143000/metadata.json | jq .
```

---

## 14. 常见问题

### Q1: 如何确定数据库类型？

工具会自动通过数据库连接执行 `SELECT version()` 来识别数据库类型。

### Q2: 如何收集特定节点的日志？

使用 `--hosts` 参数指定节点 IP，多个节点用逗号分隔。

### Q3: 如何只收集某个时间段的日志？

使用 `--start-time` 和 `--end-time` 参数指定时间范围。

### Q4: SSH 认证失败怎么办？

1. 检查用户名和密码是否正确
2. 检查 SSH 端口是否正确
3. 检查 ~/.ssh/ 目录权限是否为 700
4. 检查密钥文件权限是否为 600

### Q5: 收集速度慢怎么办？

1. 使用 `--pack-type tar` 替代 zip
2. 减少时间范围
3. 检查网络带宽

### Q6: 磁盘空间不足怎么办？

1. 清理旧的日志文件
2. 减小时间范围
3. 使用 `--output` 指定更大空间的目录

### Q7: 如何处理大集群？

对于大规模集群，建议：
1. 分批收集
2. 使用 tar 格式
3. 调整时间范围

---

## 15. 开发指南

### 15.1 项目结构

```
dbpacklogs/
├── cmd/
│   └── main.go              # 命令行入口
├── internal/
│   ├── collector/            # 日志收集器
│   │   ├── orchestrator.go   # 编排器
│   │   ├── db_collector.go  # 数据库收集
│   │   └── os_collector.go  # OS信息收集
│   ├── config/               # 配置管理
│   ├── detector/             # 数据库探测
│   │   ├── adapter.go       # 适配器接口
│   │   ├── factory.go       # 工厂函数
│   │   ├── greenplum.go     # Greenplum适配器
│   │   ├── postgres.go      # PostgreSQL适配器
│   │   └── opengauss.go     # openGauss适配器
│   ├── filter/               # 时间过滤器
│   ├── packager/            # 打包器
│   │   ├── packager.go      # 打包接口
│   │   ├── zip_packager.go  # ZIP打包
│   │   ├── tar_packager.go  # TAR打包
│   │   └── organizer.go     # 目录组织
│   ├── report/               # 报告生成
│   └── ssh/                  # SSH客户端
│       ├── client.go        # SSH连接
│       ├── pool.go         # 连接池
│       └── transfer.go      # 文件传输
└── pkg/
    └── utils/                # 工具函数
```

### 15.2 构建命令

```bash
# 构建当前平台
make build

# 运行测试
make test

# 清理构建产物
make clean

# 代码检查
make lint
```

### 15.3 添加新数据库支持

1. 在 `detector/adapter.go` 中定义适配器接口
2. 实现 `detector` 包中的适配器
3. 在 `detector/factory.go` 中添加类型识别逻辑
4. 在 `detector/*_adapter.go` 中实现节点发现逻辑

---

## 许可证

MIT License

---

## 贡献指南

欢迎提交 Issue 和 Pull Request！
