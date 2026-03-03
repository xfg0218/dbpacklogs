# DBpackLogs - Database Log Collection & Packaging Tool

English | [中文](./README_CN.md)

## Table of Contents

1. [Introduction](#1-introduction)
2. [Features](#2-features)
3. [Requirements](#3-requirements)
4. [Supported Databases](#4-supported-databases)
5. [Installation](#5-installation)
6. [Quick Start](#6-quick-start)
7. [CLI Reference](#7-cli-reference)
8. [Examples](#8-examples)
9. [Output Structure](#9-output-structure)
10. [How It Works](#10-how-it-works)
11. [Architecture](#11-architecture)
12. [Configuration Examples](#12-configuration-examples)
13. [Troubleshooting](#13-troubleshooting)
14. [FAQ](#14-faq)
15. [Development Guide](#15-development-guide)

---

## 1. Introduction

DBpackLogs is an enterprise-grade database log collection and packaging tool. It connects to database nodes via SSH and automatically:

- Detects database type (Greenplum / PostgreSQL / openGauss)
- Discovers all nodes in the cluster
- Collects database logs within a specified time range
- Collects database configuration files
- Collects OS diagnostic information
- Generates a collection report
- Packages everything into ZIP or TAR.GZ format

### Use Cases

- Database failure diagnosis and troubleshooting
- Database performance analysis
- Audit and compliance data collection
- Pre-migration data gathering
- Routine database health checks

---

## 2. Features

### 2.1 Core Features

| Feature | Description |
|---------|-------------|
| **Multi-database support** | Auto-detects Greenplum, PostgreSQL, openGauss |
| **Auto node discovery** | Discovers all cluster nodes (coordinator / primary / standby / segment) |
| **Time-range filtering** | Collects logs within a precise time window; supports multiple time formats |
| **Config file collection** | Collects `postgresql.conf`, `pg_hba.conf`, `pg_ident.conf`, etc. |
| **OS info collection** | Collects CPU, memory, disk, network, system logs, etc. |
| **Flexible packaging** | Outputs ZIP or TAR.GZ archives |
| **Fault tolerance** | A single-node failure does not interrupt collection on other nodes |

### 2.2 Advanced Features

| Feature | Description |
|---------|-------------|
| **SSH connection pool** | Reuses SSH connections to improve multi-node efficiency |
| **Exponential backoff retry** | Automatically retries on transient network errors |
| **Concurrent collection** | Collects from multiple nodes in parallel to reduce total time |
| **Graceful shutdown** | Handles Ctrl+C / SIGTERM cleanly |
| **Debug mode** | Detailed debug-level log output via `--verbose` |

---

## 3. Requirements

### 3.1 Runtime Requirements

| Requirement | Details |
|-------------|---------|
| **OS** | Linux (CentOS, RHEL, Ubuntu, Debian, etc.) |
| **Go version** | Go 1.21 or higher (for building from source) |
| **SSH access** | SSH access to all target database nodes |
| **Disk space** | Reserve sufficient space based on expected log volume |

### 3.2 Target Node Requirements

| Component | Requirement |
|-----------|-------------|
| **SSH service** | SSH must be enabled on each target node |
| **Database** | Greenplum 5.x+ / PostgreSQL 9.x+ / openGauss 3.x+ |
| **Disk space** | Sufficient free space on the partition hosting log directories |

---

## 4. Supported Databases

### 4.1 Greenplum

| Version | Status | Node Discovery |
|---------|--------|----------------|
| 5.x | ✅ Supported | `gp_segment_configuration` |
| 6.x | ✅ Supported | `gp_segment_configuration` |
| 7.x | ✅ Supported | `gp_segment_configuration` |

**Node roles**: `coordinator`, `primary`, `mirror`, `standby`, `segment`

### 4.2 PostgreSQL

| Version | Status | Node Discovery |
|---------|--------|----------------|
| 9.x | ✅ Supported | `pg_stat_replication` |
| 10.x | ✅ Supported | `pg_stat_replication` |
| 11.x – 16.x | ✅ Supported | `pg_stat_replication` |

**Node roles**: `primary`, `standby`

**Replication types**: Streaming, Logical, Physical

### 4.3 openGauss

| Version | Status | Node Discovery |
|---------|--------|----------------|
| 3.0.x | ✅ Supported | `cm_ctl` / `gs_om` |
| 3.1.x | ✅ Supported | `cm_ctl` / `gs_om` |
| 5.0.x | ✅ Supported | `cm_ctl` / `gs_om` |

**Node roles**: `primary`, `standby`, `cascade standby`

---

## 5. Installation

### 5.1 Build from Source

```bash
git clone https://github.com/xfg0218/dbpacklogs.git
cd dbpacklogs

make build

./bin/dbpacklogs --help
```

### 5.2 Cross-Compilation

```bash
make build-linux-amd64   # Linux AMD64
make build-linux-arm64   # Linux ARM64
make build-darwin-amd64  # macOS AMD64
make build-darwin-arm64  # macOS ARM64
```

### 5.3 Pre-built Binaries

Download the binary for your platform from the [Releases](https://github.com/xfg0218/dbpacklogs/releases) page.

---

## 6. Quick Start

> **Typical deployment**: DBpackLogs runs **on the master/coordinator node** of the cluster, where passwordless SSH to all data nodes is already configured and the database is accessible via peer authentication. In this setup no credentials are required at all.

```bash
# Minimal — run on the master node (uses current OS user, peer DB auth)
./bin/dbpacklogs --hosts 10.0.0.10

# All nodes in /etc/hosts (recommended for Greenplum / openGauss)
./bin/dbpacklogs --all-hosts

# With explicit time range
./bin/dbpacklogs --all-hosts --start-time '2026-02-20' --end-time '2026-02-27'

# Override SSH/DB user when running from a different account
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

## 7. CLI Reference

### 7.1 Node Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--hosts` | — | Comma-separated list of node IPs |
| `--all-hosts` | `false` | Read all IPs from `/etc/hosts` (mutually exclusive with `--hosts`) |

### 7.2 SSH Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--ssh-user` | current OS user | SSH username |
| `--ssh-port` | `22` | SSH port |
| `--ssh-password` | — | SSH password (not needed when passwordless SSH is configured) |
| `--ssh-key` | — | Path to SSH private key (optional; falls back to `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`) |
| `--insecure-hostkey` | `false` | Skip SSH host key verification (insecure; for first-time connections to unknown hosts) |

> **Note:** When connecting to a host that is not in `~/.ssh/known_hosts`, either add it first with `ssh-keyscan -H <host> >> ~/.ssh/known_hosts`, or pass `--insecure-hostkey`.

### 7.3 Database Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--db-port` | `5432` | Database port |
| `--db-user` | same as `--ssh-user` | Database username |
| `--db-password` | — | Database password (not needed with peer/trust authentication) |
| `--db-name` | `postgres` | Database name |

> The database host is automatically derived from the first entry in `--hosts`; there is no `--db-host` flag.
> When running on the master node, the OS user typically has peer access to the database and no password is required.

### 7.4 Time Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--start-time` | 3 days ago | Collection start time |
| `--end-time` | now | Collection end time |

**Supported time formats:**

| Format | Example |
|--------|---------|
| `2006-01-02 15:04:05` | `2026-02-24 08:00:00` |
| `2006-01-02T15:04:05` | `2026-02-24T08:00:00` |
| `2006-01-02` | `2026-02-24` |
| `20060102` | `20260224` |

### 7.5 Output Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `.` | Output directory |
| `--pack-type` | `zip` | Archive format: `zip` or `tar` |
| `--verbose` | `false` | Enable debug logging |

---

## 8. Examples

### 8.1 Basic Usage

```bash
# View help
./bin/dbpacklogs --help

# Single node — run on master, no credentials needed (peer auth + passwordless SSH)
./bin/dbpacklogs --hosts 10.0.0.10

# Multiple nodes
./bin/dbpacklogs --hosts 10.0.0.10,10.0.0.11,10.0.0.12

# all-hosts mode (reads from /etc/hosts)
./bin/dbpacklogs --all-hosts

# Custom output directory
./bin/dbpacklogs --hosts 10.0.0.10 --output /data/backup
```

### 8.2 Time Range Filtering

```bash
# Date range
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-20' --end-time '2026-02-25'

# Single day
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24' --end-time '2026-02-25'

# With time-of-day precision
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24 08:00:00' --end-time '2026-02-24 20:00:00'

# ISO format
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '2026-02-24T00:00:00' --end-time '2026-02-25T00:00:00'

# Compact date format
./bin/dbpacklogs --hosts 10.0.0.10 \
  --start-time '20260224' --end-time '20260225'
```

### 8.3 Authentication

```bash
# SSH password (when passwordless SSH is not configured)
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-password 'P@ssw0rd'

# SSH private key
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-user gpadmin --ssh-key ~/.ssh/id_rsa

# Skip host key check (first-time / unknown host)
./bin/dbpacklogs --hosts 10.0.0.10 --insecure-hostkey

# Custom SSH port
./bin/dbpacklogs --hosts 10.0.0.10 --ssh-port 2222
```

### 8.4 Archive Format

```bash
# ZIP (default)
./bin/dbpacklogs --hosts 10.0.0.10 --pack-type zip

# TAR.GZ
./bin/dbpacklogs --hosts 10.0.0.10 --pack-type tar
```

### 8.5 Database-Specific

```bash
# Greenplum cluster — run on coordinator, no credentials needed
./bin/dbpacklogs --all-hosts --db-port 5432 --db-name gpadmin

# PostgreSQL primary/standby — override user if needed
./bin/dbpacklogs --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user postgres --db-port 5432

# openGauss cluster — override user if needed
./bin/dbpacklogs --all-hosts --ssh-user omm \
  --db-port 5432 --db-user gaussdb
```

### 8.6 Full Example

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

## 9. Output Structure

### 9.1 Directory Layout

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

### 9.2 Collection Report Sample

```
=== DBpackLogs Collection Report ===
Generated At  : 2026-02-27 14:30:00
Database Type : greenplum
Time Range    : 2026-02-20 00:00:00 ~ 2026-02-27 14:30:00
Total Nodes   : 3
Success Nodes : 3
Failed Nodes  : 0
Total Duration: 2m30s

--- Success Nodes ---
  [OK] 10.0.0.10           role=coordinator  elapsed=45.2s
  [OK] 10.0.0.11           role=primary      elapsed=42.1s
  [OK] 10.0.0.12           role=standby      elapsed=38.5s
```

### 9.3 Metadata JSON Sample

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

## 10. How It Works

### 10.1 Execution Flow

```
1. Initialization
   ├── Parse CLI flags
   ├── Init logger
   ├── Validate parameters
   ├── Create work directory
   └── Init SSH connection pool

2. Database Detection
   ├── Connect to the first node via pgx
   ├── Execute SELECT version()
   └── Route to the matching adapter (Greenplum / PostgreSQL / openGauss)

3. Node Discovery
   ├── Greenplum : gp_segment_configuration
   ├── PostgreSQL: pg_stat_replication + current_setting('data_directory')
   └── openGauss : cm_ctl query -Cv  →  gs_om -t status --detail  →  single-node fallback

4. Concurrent Collection (errgroup + semaphore, max 10 parallel)
   └── Per node:
       ├── SSH connect (pool, keepalive, exponential-backoff retry)
       ├── Resolve data_directory (once, pointer write-back)
       ├── Download DB config files (SFTP)
       ├── Stream DB logs as tar.gz (remote tar → SSH stdout → local file)
       └── Collect OS info (cpu / memory / disk / network / dmesg / journalctl / raid)

5. Report Generation
   ├── collection_report.txt  (human-readable)
   └── metadata.json          (machine-readable)

6. Packaging & Cleanup
   ├── Pack work directory → ZIP or TAR.GZ
   └── Remove temporary work directory
```

### 10.2 Database Type Detection

```go
switch {
case strings.Contains(version, "greenplum"):  // → GreenplumAdapter
case strings.Contains(version, "opengauss"),
     strings.Contains(version, "gaussdb"):    // → OpenGaussAdapter
case strings.Contains(version, "postgresql"): // → PostgresAdapter
}
```

### 10.3 Node Discovery

**Greenplum:**
```sql
SELECT address, port, role, preferred_role, datadir
FROM gp_segment_configuration
WHERE status = 'u'
ORDER BY content, role
```

**PostgreSQL:**
```sql
-- Primary
SELECT current_setting('data_directory')

-- Standbys
SELECT client_addr, application_name
FROM pg_stat_replication
WHERE state = 'streaming'
```

**openGauss:**
```bash
cm_ctl query -Cv          # preferred
gs_om -t status --detail  # fallback
```

---

## 11. Architecture

### 11.1 Module Layout

```
┌──────────────────────────────────────────────────┐
│                   cmd/main.go                    │
│               CLI entry point                    │
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
│  └───────��──┘  └──────────┘  └──────────┘       │
└──────────────────────────────────────────────────┘
```

### 11.2 Module Responsibilities

| Module | Responsibility |
|--------|---------------|
| `cmd` | CLI entry point, flag parsing |
| `collector` | Orchestrates the full collection pipeline |
| `detector` | DB type detection and node discovery |
| `filter` | Time-range filtering for logs and dmesg |
| `packager` | ZIP / TAR.GZ packaging and directory organization |
| `report` | Text report and JSON metadata generation |
| `ssh` | SSH client, connection pool, SFTP transfer, remote tar stream |
| `config` | Config struct, validation, `/etc/hosts` parsing |
| `utils` | Logger, time parsing, byte/duration formatting |

---

## 12. Configuration Examples

### 12.1 Greenplum Cluster (run on coordinator)

```bash
# Minimal — no credentials needed (passwordless SSH + peer auth)
./bin/dbpacklogs \
  --all-hosts \
  --start-time '2026-02-20' \
  --end-time '2026-02-27' \
  --pack-type tar \
  --output /data/gp_backup

# With explicit credentials
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

### 12.2 PostgreSQL Primary/Standby

```bash
./bin/dbpacklogs \
  --hosts 10.0.0.10,10.0.0.11 \
  --ssh-user postgres \
  --start-time '2026-02-24' \
  --end-time '2026-02-25' \
  --pack-type zip \
  --output /data/pg_backup
```

### 12.3 openGauss Cluster

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

## 13. Troubleshooting

### 13.1 Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `SSH connection refused` | SSH not running or wrong port | Check SSH service and `--ssh-port` |
| `Authentication failed` | Wrong username or credential | Verify `--ssh-user`, `--ssh-password`, or `--ssh-key` |
| `host not in known_hosts` | First connection to this node | Use `--insecure-hostkey` or run `ssh-keyscan -H <host> >> ~/.ssh/known_hosts` |
| `Database connection failed` | Wrong DB params | Check `--db-port`, `--db-user`, `--db-password`, `--db-name` |
| `Permission denied` | Insufficient SSH user privileges | Verify SSH user has read access to DB data/log directories |
| Output directory error | Path does not exist or is not writable | Specify a valid writable directory with `--output` |

### 13.2 Debug Mode

```bash
./bin/dbpacklogs --hosts 10.0.0.10 --verbose
```

### 13.3 Reviewing the Report

```bash
# Text report
cat DBpackLogs_20260227_143000_012345/collection_report.txt

# JSON metadata
cat DBpackLogs_20260227_143000_012345/metadata.json | jq .
```

---

## 14. FAQ

**Q1: How is the database type determined?**
The tool connects to the first node and runs `SELECT version()`. The result string is matched against `"greenplum"`, `"opengauss"` / `"gaussdb"`, and `"postgresql"`.

**Q2: How do I collect logs from specific nodes only?**
Use `--hosts` with a comma-separated list of IPs.

**Q3: How do I limit the time range?**
Pass `--start-time` and `--end-time`. The maximum range is 90 days; if exceeded, the start time is adjusted automatically.

**Q4: SSH authentication fails — what should I do?**
1. Confirm the username and password/key are correct.
2. Check the SSH port (`--ssh-port`).
3. Ensure `~/.ssh/` is `700` and key files are `600`.
4. If the host is not in `known_hosts`, use `--insecure-hostkey` or run `ssh-keyscan`.

**Q5: Collection is slow — how can I speed it up?**
1. Use `--pack-type tar` instead of `zip`.
2. Narrow the time range.
3. Check network bandwidth between the tool host and the DB nodes.

**Q6: Disk space is low — what should I do?**
1. Free up space or use `--output` to point to a larger volume.
2. Reduce the time range to collect fewer logs.

**Q7: How do I handle a large cluster?**
Collect in batches using `--hosts`, use `--pack-type tar` for better compression speed, and narrow the time window.

---

## 15. Development Guide

### 15.1 Project Structure

```
dbpacklogs/
├── cmd/
│   └── main.go                  # CLI entry point
├── internal/
│   ├── collector/
│   │   ├── orchestrator.go      # Collection pipeline orchestrator
│   │   ├── db_collector.go      # DB config & log collection
│   │   └── os_collector.go      # OS diagnostic collection
│   ├── config/
│   │   └── config.go            # Config struct, validation, /etc/hosts parsing
│   ├── detector/
│   │   ├── adapter.go           # DBAdapter interface + NodeInfo
│   │   ├── factory.go           # Auto-detection factory (SELECT version())
│   │   ├── greenplum.go         # Greenplum adapter
│   │   ├── postgres.go          # PostgreSQL adapter
│   │   └── opengauss.go         # openGauss adapter (SSH + cm_ctl/gs_om)
│   ├── filter/
│   │   └── time_filter.go       # Time-range filtering, dmesg parsing
│   ├── packager/
│   │   ├── packager.go          # Packager interface
│   │   ├── zip_packager.go      # ZIP implementation
│   │   ├── tar_packager.go      # TAR.GZ implementation
│   │   └── organizer.go         # Work-dir creation and layout
│   ├── report/
│   │   └── report.go            # Text report + JSON metadata
│   └── ssh/
│       ├── client.go            # SSH client, auth, retry
│       ├── pool.go              # SSH connection pool
│       └── transfer.go          # SFTP download, remote tar stream, remote find
└── pkg/
    └── utils/
        ├── logger.go            # Zap logger wrapper
        └── format.go            # Time parsing, byte/duration formatting
```

### 15.2 Build Commands

```bash
make build           # Build for current platform → ./bin/dbpacklogs
make build-linux-amd64
make build-linux-arm64
make test            # Run tests
make lint            # Run linter
make clean           # Remove build artifacts
```

### 15.3 Adding a New Database

1. Define the adapter in `detector/adapter.go` (implement `DBAdapter`).
2. Create `detector/<dbname>.go` with `Detect()`, `DiscoverNodes()`, `GetLogPaths()`.
3. Register the new type in `detector/factory.go` by matching the `version()` string.
4. Add the `DBType` constant to `detector/adapter.go`.

---

## License

MIT License

---

## Contributing

Issues and pull requests are welcome!
