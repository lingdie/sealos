# Devbox 命令工具

本目录包含了用于管理 Devbox 生命周期的各种命令行工具。

## 命令概览

### 1. devbox-pause (已弃用)
**位置**: `cmd/devbox-pause/`
**状态**: 已弃用，建议使用独立的命令

原始的暂停命令，可以同时暂停 devboxes 和 controller。为了更好的控制和模块化，建议使用下面的独立命令。

```bash
# 传统用法（已弃用）
./devbox-pause --dry-run
./devbox-pause --only-devboxes  # 只暂停 devboxes
./devbox-pause --only-controller  # 只暂停 controller
```

### 2. devbox-stop (推荐)
**位置**: `cmd/devbox-stop/`
**功能**: 专门用于将所有 devbox 变为 stop 状态

```bash
# 停止所有 devboxes
./devbox-stop --dry-run

# 停止特定 namespace 的 devboxes
./devbox-stop --namespace my-namespace

# 设置备份目录
./devbox-stop --backup-dir ./my-backup

# 设置 commit 超时时间
./devbox-stop --commit-timeout 10m
```

**参数说明**:
- `--dry-run`: 仅显示将要执行的操作，不实际执行
- `--namespace`: 指定要操作的 namespace，留空则操作所有 namespace
- `--backup-dir`: 备份文件存储目录，默认为 `./backup`
- `--commit-timeout`: 等待 commit 完成的超时时间，默认为 5 分钟

### 3. controller-pause (推荐)
**位置**: `cmd/controller-pause/`
**功能**: 专门用于暂停 devbox controller

```bash
# 暂停 controller
./controller-pause --dry-run

# 指定 controller 信息
./controller-pause --controller-namespace devbox-system \
                  --controller-name devbox-controller-manager

# 设置备份目录
./controller-pause --backup-dir ./my-backup
```

**参数说明**:
- `--dry-run`: 仅显示将要执行的操作，不实际执行
- `--controller-namespace`: controller 所在的 namespace，默认为 `devbox-system`
- `--controller-name`: controller deployment 的名称，默认为 `devbox-controller-manager`
- `--backup-dir`: 备份文件存储目录，默认为 `./backup`

## 推荐的使用流程

### 完整暂停流程
```bash
# 1. 首先停止所有 devboxes
./devbox-stop --backup-dir ./upgrade-backup

# 2. 然后暂停 controller
./controller-pause --backup-dir ./upgrade-backup
```

### 只停止 devboxes
```bash
./devbox-stop --namespace production --backup-dir ./prod-backup
```

### 只暂停 controller
```bash
./controller-pause --backup-dir ./controller-backup
```

## 备份文件

每个命令都会在指定的备份目录中创建相应的备份文件：

- **devbox-stop**: 创建 `devbox_backup_states.json` 文件，记录所有 devbox 的原始状态
- **controller-pause**: 创建 `controller_deployment.yaml` 文件，备份 controller deployment 配置

## 公共模块

所有命令共享 `cmd/devbox-pause/common/` 中的公共函数：
- `types.go`: 定义公共数据结构
- `utils.go`: 提供公共工具函数

## 注意事项

1. **权限要求**: 所有命令都需要足够的 Kubernetes 权限来操作相应的资源
2. **备份重要性**: 在执行任何操作前，建议先使用 `--dry-run` 参数预览操作
3. **操作顺序**: 在升级场景中，建议先停止 devboxes，再暂停 controller
4. **恢复操作**: 备份文件可用于后续的恢复操作

## 兼容性

- 支持 v1alpha1 和 v1alpha2 版本的 Devbox CRD
- 兼容 Kubernetes 1.20+ 版本
- 支持跨 namespace 操作
