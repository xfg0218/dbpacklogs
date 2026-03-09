# DBpackLogs - 数据库日志收集打包工具

[English](./README.md) | 中文

## 目录

1. [项目介绍](#1-项目介绍)
2. [功能特性](#2-功能特性)
3. [系统要求](#3-系统要求)
4. [支持的数据库](#4-支持的数据库)
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

- 自动识别数据库类型（Greenplum / PostgreSQL / openGauss）
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
| **自动节点发现** | 自动发现集群中的所有节点（coordinator / primary / standby / segment） |
| **时间范围过滤** | 按指定时间范围精确收集日志，支持多种时间格式 |
| **配置文件收集** | 自动收集 `postgresql.conf`、`pg_hba.conf`、`pg_ident.conf` 等 |
| **OS 信息收集** | 收集 CPU、内存、磁盘、网络、系统日志等 |
| **智能打包** | 支持 ZIP 和 TAR.GZ 格式输出 |
| **错误容忍** | 单节点失败不影响其他节点收集 |

### 2.2 高级特性

| 特性 | 说明 |
|------|------|
| **SSH 连接池** | 复用 SSH 连接，提升多节点收集效率 |
| **指数退避重试** | 网络错误自动重试，避免瞬时故障 |
| **并发收集** | 多节点并行收集（最大并发 10），缩短总耗时 |
| **信号处理** | 支持 Ctrl+C / SIGTERM 优雅退出 |
| **调试模式** | `--verbose` 开启详细的 DEBUG 日志输出 |

---

## 3. 系统要求

### 3.1 运行要求

| 要求 | 说明 |
|------|------|
| **操作系统** | Linux（CentOS、RHEL、Ubuntu、Debian 等） |
| **Go 版本** | Go 1.21 或更高版本（源码构建时需要） |
| **SSH 访问** | 能够 SSH 访问所有目标数据库节点 |
| **磁盘空间** | 根据日志量预留足够空间 |

### 3.2 目标节点要求

| 组件 | 要求 |
|------|------|
| **SSH 服务** | 目标节点需要开启 SSH 服务 |
| **数据库** | Greenplum 5.x+ / PostgreSQL 9.x+ / openGauss 3.x+ |
| **磁盘空间** | 日志目录所在分区需有足够可用空间 |

---

## 4. 支持的数据库

### 4.1 Greenplum

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 5.x | ✅ 支持 | `gp_segment_configuration` |
| 6.x | ✅ 支持 | `gp_segment_configuration` |
| 7.x | ✅ 支持 | `gp_segment_configuration` |

**节点角色**：`coordinator`、`primary`、`mirror`、`standby`、`segment`

### 4.2 PostgreSQL

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 9.x | ✅ 支持 | `pg_stat_replication` |
| 10.x | ✅ 支持 | `pg_stat_replication` |
| 11.x - 16.x | ✅ 支持 | `pg_stat_replication` |

**节点角色**：`primary`、`standby`

**复制方式**：流复制、逻辑复制、物理复制

### 4.3 openGauss

| 版本 | 支持状态 | 节点发现方式 |
|------|----------|--------------|
| 3.0.x | ✅ 支持 | `cm_ctl` / `gs_om` |
| 3.1.x | ✅ 支持 | `cm_ctl` / `gs_om` |
| 5.0.x | ✅ 支持 | `cm_ctl` / `gs_om` |

**节点角色**：`primary`、`standby`、`cascade standby`

---

## 5. 安装指南

### 5.1 从源码构建

```bash
git clone https://github.com/xfg0218/dbpacklogs.git
cd dbpacklogs

make build

./bin/dbpacklogs --help
```

### 5.2 交叉编译

```bash
make build-linux-amd64   # Linux AMD64
make build-linux-arm64   # Linux ARM64
make build-darwin-amd64  # macOS AMD64
make build-darwin-arm64  # macOS ARM64
```

### 5.3 预编译版本

从 [Releases](https://github.com/xfg0218/dbpacklogs/releases) 页面下载对应平台的二进制文件。

---

## 6. 快速开始

> **典型部署场景**：DBpackLogs 在集群的 **master/coordinator 节点**上运行，该节点通常已对所有数据节点配置了免密 SSH，同时数据库支持 peer 认证，因此无需任何凭据即可直接运行。

```bash
# 最简用法——在 master 节点运行（使用当前 OS 用户，peer 认证连接数据库）
./bin/dbpacklogs --hosts 10.0.0.10

# 自动读取 /etc/hosts 中的所有节点（推荐用于 Greenplum / openGauss）
./bin/dbpacklogs --all-hosts

# 指定时间范围
./bin/dbpacklogs --all-hosts --start-time '2026-02-20' --end-time '2026-02-27'

# 非当前用户运行时手动指定 SSH/DB 用户
./bin/dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11,10.0.0.12 \
  --ssh-user gpadmin \
  --db-user gpadmin \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/backup \
  --verbose
```

---

## 7. 命令参数详解

### 7.1 节点参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--hosts` | — | 指定节点列表，逗号分隔 |
| `--all-hosts` | `false` | 从 `/etc/hosts` 读取所有节点（与 `--hosts` 互斥） |

### 7.2 SSH 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--ssh-user` | 当前 OS 用户 | SSH 用户名 |
| `--ssh-port` | `22` | SSH 端口 |
| `--ssh-password` | — | SSH 密码（免密 SSH 时无需指定） |
| `--ssh-key` | — | SSH 私钥路径（可选；未指定时自动尝试 `~/.ssh/id_rsa`、`~/.ssh/id_ed25519`） |
| `--insecure-hostkey` | `false` | 跳过 SSH 主机密钥校验（首次连接未知主机时使用） |

> **提示：** 首次连接新节点时，若该主机不在 `~/.ssh/known_hosts` 中，可先执行 `ssh-keyscan -H <host> >> ~/.ssh/known_hosts`，或使用 `--insecure-hostkey` 跳过校验。

### 7.3 数据库参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--db-port` | `5432` | 数据库端口 |
| `--db-user` | 与 `--ssh-user` 一致 | 数据库用户名 |
| `--db-password` | — | 数据库密码（peer/trust 认证时无需指定） |
| `--db-name` | `postgres` | 数据库名称 |

> 数据库连接地址由 `--hosts` 的第一个节点自动推导，无需指定 `--db-host`。
> 在 master 节点上运行时，OS 用户通常已通过 peer 认证直连数据库，无需密码。

### 7.4 时间参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--start-time` | 最近 3 天 | 收集起始时间 |
| `--end-time` | 当前时间 | 收集结束时间 |

**支持的时间格式：**

| 格式 | 示例 |
|------|------|
| `2006-01-02 15:04:05` | `2026-02-24 08:00:00` |
| `2006-01-02T15:04:05` | `2026-02-24T08:00:00` |
| `2006-01-02` | `2026-02-24` |
| `20060102` | `20260224` |

### 7.5 输出参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--output` | `.` | 输出目录 |
| `--pack-type` | `zip` | 打包类型：`zip` 或 `tar` |
| `--verbose` | `false` | 启用调试模式（显示 DEBUG 日志） |

---

## 8. 使用示例

### 8.1 基础使用

```bash
# 查看帮助
./bin/dbpacklogs --help

# 单节点收集（在 master 节点运行，无需凭据）
./bin/dbpacklogs --hosts 10.0.0.10

# 多节点收集
./bin/dbpacklogs --hosts 10.0.0.10,10.0.0.11,10.0.0.12

# all-hosts 模式（从 /etc/hosts 读取）
./bin/dbpacklogs --all-hosts

# 指定输出目录
./bin/dbpacklogs --hosts 10.0.0.10 --output /data/backup
```

### 8.2 时间范围过滤

```bash
# 日期范围
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-20' --end-time '2026-02-25'

# 单日日志
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24' --end-time '2026-02-25'

# 带时分秒
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24 08:00:00' --end-time '2026-02-24 20:00:00'

# ISO 格式
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24T00:00:00' --end-time '2026-02-25T00:00:00'

# 紧凑日期格式
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '20260224' --end-time '20260225'
```

### 8.3 认证方式

```bash
# SSH 密码（未配置免密 SSH 时使用）
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-password 'P@ssw0rd'

# SSH 私钥
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-key ~/.ssh/id_rsa

# 跳过主机密钥校验（首次连接未知主机）
./bin/dbpacklogs --hosts 10.0.0.10 --insecure-hostkey

# 自定义 SSH 端口
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-port 2222
```

### 8.4 打包格式

```bash
# ZIP（默认）
./bin/dbpacklogs --hosts 10.0.0.10 --pack-type zip

# TAR.GZ
./bin/dbpacklogs --hosts 10.0.0.10 --pack-type tar
```

### 8.5 数据库特定场景

```bash
# Greenplum 集群——在 coordinator 节点运行，无需凭据
./bin/dbpacklogs --all-hosts --db-port 5432 --db-name gpadmin

# PostgreSQL 主从——需要时覆盖用户名
./bin/dbpacklogs --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user postgres --db-port 5432

# openGauss 集群——指定运维用户
./bin/dbpacklogs --all-hosts --ssh-user omm \
  --db-port 5432 --db-user gaussdb
```

### 8.6 完整参数示例

```bash
./bin/dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user gpadmin \
  --db-port 5432 \
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
└── DBpackLogs_20260227_143000_012345/
    ├── greenplum/
    │   ├── 10.0.0.10/
    │   │   ├── db_info/
    │   │   │   ├── postgresql.conf
    │   │   │   ├── pg_hba.conf
    │   │   │   ├── pg_ident.conf
    │   │   │   └── cluster_topology.txt
    │   │   ├── db_logs/
    │   │   │   └── pg_log_10_0_0_10_pg_log.tar.gz
    │   │   └── os_info/
    │   │       ├── cpu.txt
    │   │       ├── memory.txt
    │   │       ├── disk.txt
    │   │       ├── network.txt
    │   │       ├── dmesg.txt
    │   │       ├── journalctl.txt
    │   │       ├── raid.txt
    │   │       └── os_info.txt
    │   └── 10.0.0.11/
    │       └── ...
    ├── collection_report.txt
    └── metadata.json
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
  [OK] 10.0.0.10           role=coordinator  elapsed=45.2s
  [OK] 10.0.0.11           role=primary      elapsed=42.1s
  [OK] 10.0.0.12           role=standby      elapsed=38.5s
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
1. 初始化
   ├── 解析命令行参数
   ├── 初始化日志系统
   ├── 验证参数合法性
   ├── 创建工作目录
   └── 初始化 SSH 连接池

2. 数据库探测
   ├── 通过 pgx 连接第一个节点
   ├── 执行 SELECT version()
   └── 根据返回结果路由到对应适配器（Greenplum / PostgreSQL / openGauss）

3. 节点发现
   ├── Greenplum : gp_segment_configuration
   ├── PostgreSQL: pg_stat_replication + current_setting('data_directory')
   └── openGauss : cm_ctl query -Cv → gs_om -t status --detail → 单节点回退

4. 并发收集（errgroup + semaphore，最大并发 10）
   └── 每个节点：
       ├── SSH 连接（连接池 + keepalive + 指数退避重试）
       ├── 解析 data_directory（统一执行一次，指针写回）
       ├── 通过 SFTP 下载 DB 配置文件
       ├── 远端 tar 流式传输 DB 日志
       └── 收集 OS 信息（cpu / memory / disk / network / dmesg / journalctl / raid）

5. 生成报告
   ├── collection_report.txt（人类可读）
   └── metadata.json（机器可读）

6. 打包与清理
   ├── 将工作目录打包为 ZIP 或 TAR.GZ
   └── 删除临时工作目录
```

### 10.2 数据库类型识别

```go
switch {
case strings.Contains(version, "greenplum"):  // → GreenplumAdapter
case strings.Contains(version, "opengauss"),
     strings.Contains(version, "gaussdb"):    // → OpenGaussAdapter
case strings.Contains(version, "postgresql"): // → PostgresAdapter
}
```

### 10.3 节点发现机制

**Greenplum：**
```sql
SELECT address, port, role, preferred_role, datadir
FROM gp_segment_configuration
WHERE status = 'u'
ORDER BY content, role
```

**PostgreSQL：**
```sql
-- 主节点
SELECT current_setting('data_directory')

-- Standby 节点
SELECT client_addr, application_name
FROM pg_stat_replication
WHERE state = 'streaming'
```

**openGauss：**
```bash
cm_ctl query -Cv           # 优先
gs_om -t status --detail   # 回退
```

---

## 11. 架构设计

### 11.1 模块架构

```
┌──────────────────────────────────────────────────┐
│                   cmd/main.go                    │
│                  命令行入口层                     │
└──────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────┐
│          collector / orchestrator                │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────┐  │
│  │DBCollector  │ │OSCollector  │ │ Reporter  │  │
│  └─────────────┘ └─────────────┘ └───────────┘  │
└──────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────┐
│                   detector                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │Greenplum │  │Postgres  │  │OpenGauss │       │
│  └──────────┘  └──────────┘  └──────────┘       │
└──────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────┐
│                     ssh                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │  Client  │  │   Pool   │  │ Transfer │       │
│  └──────────┘  └──────────┘  └──────────┘       │
└──────────────────────────────────────────────────┘
```

### 11.2 核心模块说明

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

---

## 12. 配置示例

### 12.1 Greenplum 集群（在 coordinator 节点运行）

```bash
# 最简用法——免密 SSH + peer 认证，无需任何凭据
./bin/dbpacklogs \
  --all-hosts \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/gp_backup

# 显式指定凭据
./bin/dbpacklogs \
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
./bin/dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user postgres \
  --start-time '2026-02-24' \
  --end-time '2026-02-25' \
  --pack-type zip \
  --output /data/pg_backup
```

### 12.3 openGauss 集群

```bash
./bin/dbpacklogs \
  --all-hosts \
  --ssh-user omm \
  --db-user gaussdb \
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
| `SSH connection refused` | SSH 服务未启动或端口错误 | 检查 SSH 服务和 `--ssh-port` |
| `Authentication failed` | 用户名或密码错误 | 验证 `--ssh-user`、`--ssh-password`、`--ssh-key` |
| `host not in known_hosts` | 首次连接该主机 | 使用 `--insecure-hostkey` 或执行 `ssh-keyscan -H <host> >> ~/.ssh/known_hosts` |
| `Database connection failed` | 数据库参数错误 | 检查 `--db-port`、`--db-user`、`--db-password`、`--db-name` |
| `Permission denied` | SSH 用户权限不足 | 确认 SSH 用户对 DB 数据/日志目录有读权限 |
| 输出目录错误 | 路径不存在或不可写 | 使用 `--output` 指定合法的可写目录 |

### 13.2 调试模式

```bash
./bin/dbpacklogs --hosts 10.0.0.10 --verbose
```

### 13.3 日志分析

```bash
# 查看文本报告
cat DBpackLogs_20260227_143000_012345/collection_report.txt

# 查看 JSON 元数据
cat DBpackLogs_20260227_143000_012345/metadata.json | jq .
```

---

## 14. 常见问题

**Q1：如何确定数据库类型？**
工具连接第一个节点后执行 `SELECT version()`，根据返回字符串匹配 `"greenplum"`、`"opengauss"` / `"gaussdb"` 或 `"postgresql"` 来识别类型。

**Q2：如何只收集特定节点的日志？**
使用 `--hosts` 参数，多个节点 IP 用逗号分隔。

**Q3：如何限制时间范围？**
传入 `--start-time` 和 `--end-time`。最大支持 90 天范围，超出时起始时间会自动调整。

**Q4：SSH 认证失败怎么办？**
1. 确认用户名和密码/密钥正确。
2. 检查 SSH 端口（`--ssh-port`）。
3. 确保 `~/.ssh/` 权限为 `700`，密钥文件权限为 `600`。
4. 若主机不在 known_hosts 中，使用 `--insecure-hostkey` 或执行 `ssh-keyscan`。

**Q5：收集速度慢怎么办？**
1. 使用 `--pack-type tar` 替代 zip（压缩速度更快）。
2. 缩短时间范围，减少日志量。
3. 检查工具主机与 DB 节点之间的网络带宽。

**Q6：磁盘空间不足怎么办？**
1. 使用 `--output` 指向空间更大的磁盘分区。
2. 缩短时间范围，减少收集的日志文件数量。

**Q7：如何处理大规模集群？**
建议分批使用 `--hosts` 收集，使用 `--pack-type tar`，并缩短时间窗口。

---

## 15. 开发指南

### 15.1 项目结构

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

### 15.2 构建命令

```bash
make build           # 构建当前平台 → ./bin/dbpacklogs
make build-linux-amd64
make build-linux-arm64
make test            # 运行测试
make lint            # 运行代码检查
make clean           # 清理构建产物
```

### 15.3 测试命令

```bash
make test                    # 运行所有测试
go test ./...               # 运行所有测试 (替代方案)
go test ./internal/...      # 仅运行 internal 包的测试
go test -v ./pkg/utils/     # 运行特定包的测试并显示详细输出
go test -run TestFormatBytes ./pkg/utils/  # 运行特定测试函数
```

> **注意**: 某些测试可能需要适当的 SSH 连接或数据库实例才能正常运行。如果测试超时，可能是由于外部依赖。对于不依赖外部资源的纯单元测试，使用特定包路径（例如 `go test ./pkg/utils/`）。

### 15.3 添加新数据库支持

1. 在 `detector/adapter.go` 中添加新的 `DBType` 常量。
2. 新建 `detector/<dbname>.go`，实现 `Detect()`、`DiscoverNodes()`、`GetLogPaths()` 三个接口方法。
3. 在 `detector/factory.go` 中通过匹配 `version()` 返回值注册新类型。

---

## 许可证

MIT License

---

## 贡献指南

欢迎提交 Issue 和 Pull Request！
