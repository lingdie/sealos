# Controller管理工具

## 概述

`devbox-controller` 是一个专门用于管理Devbox Controller的独立工具，从原来的 `devbox-pause` 工具中拆分出来，提供更精细的控制。

## 功能特性

### 支持的操作

- **pause**: 暂停controller（删除deployment并备份）
- **resume**: 恢复controller（从备份恢复deployment）
- **status**: 查看controller状态
- **restart**: 重启controller

### 主要特点

- 独立管理controller生命周期
- 自动备份和恢复deployment配置
- 支持dry-run模式
- 详细的状态报告
- 等待操作完成确认

## 使用方法

### 基本命令

```bash
# 查看controller状态
./bin/devbox-controller --action=status

# 暂停controller
./bin/devbox-controller --action=pause

# 恢复controller
./bin/devbox-controller --action=resume

# 重启controller
./bin/devbox-controller --action=restart
```

### 选项说明

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `--action` | `status` | 操作类型：pause, resume, status, restart |
| `--controller-namespace` | `devbox-system` | Controller所在的命名空间 |
| `--controller-name` | `devbox-controller-manager` | Controller deployment名称 |
| `--backup-dir` | `./backup` | 备份文件存储目录 |
| `--wait-timeout` | `2m0s` | 等待操作完成的超时时间 |
| `--dry-run` | `false` | 只显示会执行的操作，不实际执行 |

### 详细示例

#### 1. 检查Controller状态

```bash
$ ./bin/devbox-controller --action=status
Controller Status: RUNNING
Deployment: devbox-system/devbox-controller-manager
Desired Replicas: 2
Ready Replicas: 2
Available Replicas: 2
Updated Replicas: 2

Conditions:
  Progressing: True (NewReplicaSetAvailable)
  Available: True (MinimumReplicasAvailable)
```

#### 2. 暂停Controller

```bash
# 预览操作
./bin/devbox-controller --action=pause --dry-run

# 执行暂停
./bin/devbox-controller --action=pause --backup-dir=./my-backup
```

暂停操作会：
- 备份当前deployment配置到指定目录
- 删除deployment
- 等待pods终止

#### 3. 恢复Controller

```bash
# 从备份恢复
./bin/devbox-controller --action=resume --backup-dir=./my-backup
```

恢复操作会：
- 查找最新的备份文件
- 重新创建deployment
- 等待deployment就绪

#### 4. 重启Controller

```bash
./bin/devbox-controller --action=restart
```

重启操作会：
- 更新deployment的重启注解
- 触发滚动更新
- 等待新pods就绪

## 与升级流程集成

### 在升级脚本中使用

升级脚本现在将devbox和controller的暂停分为两个独立步骤：

```bash
# Step 2a: 暂停devboxes
./bin/devbox-pause --backup-dir=$BACKUP_DIR

# Step 2b: 暂停controller
./bin/devbox-controller --action=pause --backup-dir=$BACKUP_DIR
```

### 升级完成后恢复

```bash
# 恢复controller
./bin/devbox-controller --action=resume --backup-dir=$BACKUP_DIR

# 恢复devboxes（如果需要）
./bin/devbox-restore --backup-dir=$BACKUP_DIR
```

## 故障排除

### 常见问题

1. **Controller不存在**
   ```
   Controller Status: PAUSED (deployment not found)
   ```
   - 可能已经被暂停或删除
   - 使用 `--action=resume` 恢复

2. **备份文件不存在**
   ```
   failed to find backup file: no backup files found matching pattern
   ```
   - 检查backup-dir路径是否正确
   - 确保之前执行了pause操作

3. **等待超时**
   ```
   timeout waiting for controller deployment to be ready
   ```
   - 增加 `--wait-timeout` 值
   - 检查集群资源是否充足
   - 查看deployment events

### 调试信息

启用详细日志：
```bash
./bin/devbox-controller --action=status --zap-log-level=debug
```

检查deployment状态：
```bash
kubectl get deployment devbox-controller-manager -n devbox-system -o yaml
```

查看pods状态：
```bash
kubectl get pods -n devbox-system -l app=devbox-controller-manager
```

## 最佳实践

### 1. 升级前准备

```bash
# 检查当前状态
./bin/devbox-controller --action=status

# 确保有足够的备份空间
df -h ./backup
```

### 2. 安全操作

```bash
# 总是先dry-run
./bin/devbox-controller --action=pause --dry-run

# 指定明确的备份目录
./bin/devbox-controller --action=pause --backup-dir=./backup-$(date +%Y%m%d_%H%M%S)
```

### 3. 监控操作

```bash
# 在另一个终端监控pods
watch -n 2 'kubectl get pods -n devbox-system'

# 监控deployment状态
watch -n 5 './bin/devbox-controller --action=status'
```

### 4. 自动化脚本

```bash
#!/bin/bash
set -e

BACKUP_DIR="./backup-$(date +%Y%m%d_%H%M%S)"

echo "Pausing controller..."
./bin/devbox-controller --action=pause --backup-dir="$BACKUP_DIR"

echo "Performing maintenance..."
# 执行维护操作

echo "Resuming controller..."
./bin/devbox-controller --action=resume --backup-dir="$BACKUP_DIR"

echo "Controller management completed!"
```

## 备份文件格式

备份文件命名规则：
- `controller_deployment_<timestamp>.yaml`
- 示例：`controller_deployment_1694172123.yaml`

备份文件包含完整的deployment配置，可以手动查看和编辑：
```bash
cat backup/controller_deployment_1694172123.yaml
```

## 集成其他工具

### 与Makefile集成

```makefile
.PHONY: pause-controller
pause-controller:
	./bin/devbox-controller --action=pause

.PHONY: resume-controller
resume-controller:
	./bin/devbox-controller --action=resume
```

### 与CI/CD集成

在CI/CD流程中使用：
```yaml
steps:
  - name: Pause Controller
    run: ./bin/devbox-controller --action=pause --backup-dir=${{ runner.temp }}/backup

  - name: Deploy Updates
    run: kubectl apply -f manifests/

  - name: Resume Controller
    run: ./bin/devbox-controller --action=resume --backup-dir=${{ runner.temp }}/backup
```

通过这个独立的controller管理工具，您可以更精确地控制升级过程中的每个步骤，提高升级的安全性和可靠性。
