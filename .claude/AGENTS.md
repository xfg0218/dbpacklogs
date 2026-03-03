# Agent Teams 使用指南

本项目配置了 4 个专门的 agents，用于不同的开发任务。

## 快速开始

使用启动脚本快速启动带有 agent 的 Claude Code 会话：

```bash
# 进入项目目录
cd /Users/xiaoxu/CodeBuddy/20260225104232/dbpacklogs

# 使用 refactor agent
./.claude/start-with-agents.sh refactor

# 使用 doc-updater agent
./.claude/start-with-agents.sh doc-updater

# 使用 test-writer agent
./.claude/start-with-agents.sh test-writer

# 使用 code-reviewer agent
./.claude/start-with-agents.sh code-reviewer
```

## 可用的 Agents

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
**用途**: 代码质量、安全性和性能审查

**职责**:
- 检查命令注入漏洞（特别是 SSH 命令）
- 验证所有用户输入和文件路径
- 确保正确的错误处理
- 检查硬编码凭证或密钥
- 遵循 Go 最佳实践

## 项目特定上下文

所有 agents 都了解以下项目信息：
- Go 1.24 项目
- 数据库支持: Greenplum / PostgreSQL / openGauss
- SSH 远程操作
- 最大并发节点数: 10
- 最大时间范围: 3 天
- 关键包: detector, collector, packager, report, filter, config

## 手动启动方法

如果不使用启动脚本，可以手动使用 `--agents` 和 `--agent` 参数：

```bash
claude --agents '{"refactor": {...}, "doc-updater": {...}}' --agent refactor
```

详细配置格式请参考 `.claude/AGENTS_CONFIG.md`。

## Agent 定义文件

每个 agent 的详细定义在 `.claude/agents/` 目录下：
- `refactor.md` - Refactoring Agent 定义
- `doc-updater.md` - Documentation Updater 定义
- `test-writer.md` - Test Writer 定义
- `code-reviewer.md` - Code Reviewer 定义

这些定义文件已经被整合到 `start-with-agents.sh` 启动脚本中。
