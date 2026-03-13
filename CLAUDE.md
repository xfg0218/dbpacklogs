# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build & Installation
- Build for current platform: `make build`
- Cross-compile:
  - Linux AMD64: `make build-linux-amd64`
  - Linux ARM64: `make build-linux-arm64`
  - macOS AMD64: `make build-darwin-amd64`
  - macOS ARM64: `make build-darwin-arm64`
- Run tests: `make test`
- Run linter: `make lint`

### Common Execution Patterns
- Greenplum cluster (coordinator node):
  ```bash
  ./bin/dbpacklogs --all-hosts --start-time '2026-02-20' --end-time '2026-02-27' --pack-type tar --output /data/backup
  ```
- PostgreSQL primary/standby:
  ```bash
  ./bin/dbpacklogs --hosts 10.0.0.10,10.0.0.11 --ssh-user postgres --start-time '2026-02-24'
  ```
- Debug mode:
  ```bash
  ./bin/dbpacklogs --hosts 10.0.0.10 --verbose
  ```

## Architecture

### Core Modules
- **Detector**: Auto-identifies database type (Greenplum/PostgreSQL/openGauss) via `SELECT version()`
- **Collector**: Orchestrates collection pipeline with concurrent node processing (max 10 parallel)
- **SSH Layer**: Manages connection pool with exponential backoff retry and SFTP transfers
- **Packager**: Handles ZIP/TAR.GZ packaging with directory organization

### Execution Flow
1. Parse CLI flags → Validate parameters → Create work directory
2. Database detection → Node discovery via DB-specific queries
3. Concurrent collection per node:
   - SSH connect → Resolve data directory → Download config/logs → Collect OS diagnostics
4. Generate collection report + metadata JSON
5. Package results → Cleanup temporary files

### Critical Data Structures
- `NodeInfo`: Contains host, role, datadir, and connection details
- `Config`: Struct holding all CLI parameters with validation
- Time filtering: Implemented via `filter/time_filter.go` for log/dmesg processing