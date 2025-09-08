# Devbox升级工具使用示例

本文档提供了各种升级场景的具体示例。

## 场景1: 生产环境完整升级

```bash
# 1. 构建工具
make build-upgrade-tools

# 2. 干运行验证
./scripts/upgrade-v1-to-v2.sh --dry-run

# 3. 执行升级
./scripts/upgrade-v1-to-v2.sh

# 4. 验证结果
./bin/devbox-crd --action=check-status
kubectl get devboxes -A
kubectl get devboxreleases -A
```

## 场景2: 分步升级（更安全）

```bash
# 步骤1: 备份
./bin/devbox-backup --backup-dir=./backup-$(date +%Y%m%d)

# 步骤2: 暂停（先只暂停devboxes，保持controller运行）
./bin/devbox-pause --only-devboxes --backup-dir=./backup-$(date +%Y%m%d)

# 验证devboxes已暂停
kubectl get devboxes -A

# 步骤3: 暂停controller
./bin/devbox-pause --only-controller --backup-dir=./backup-$(date +%Y%m%d)

# 步骤4: 更新CRD
./bin/devbox-crd --action=apply --crd-dir=./scripts/manifests

# 等待CRD就绪
./bin/devbox-crd --action=check-status

# 步骤5: 转换CR
./bin/devbox-transform --batch-size=5 --delay=2s

# 步骤6: 禁用v1alpha1
./bin/devbox-crd --action=disable-v1alpha1

# 最终验证
./bin/devbox-crd --action=check-status
```

## 场景3: 特定命名空间升级

```bash
# 只升级特定命名空间
./scripts/upgrade-v1-to-v2.sh --namespace=production

# 或者手动分步
./bin/devbox-backup --namespace=production --backup-dir=./backup-prod
./bin/devbox-pause --namespace=production --only-devboxes --backup-dir=./backup-prod
./bin/devbox-transform --namespace=production
```

## 场景4: 只升级CRD，不转换数据

```bash
# 只更新CRD定义
./bin/devbox-crd --action=apply --crd-dir=./scripts/manifests

# 检查状态
./bin/devbox-crd --action=check-status
```

## 场景5: 批量转换大量资源

```bash
# 小批量，长延迟（适合大量资源）
./bin/devbox-transform --batch-size=3 --delay=5s

# 只转换devboxes
./bin/devbox-transform --only-devboxes --batch-size=5 --delay=1s

# 只转换releases
./bin/devbox-transform --only-releases
```

## 场景6: 故障恢复

### 恢复devbox状态
```bash
# 查看备份的状态
cat backup/devbox_backup_states.json

# 恢复所有devbox状态
./bin/devbox-restore --backup-dir=./backup

# 恢复特定操作的状态
./bin/devbox-restore --backup-dir=./backup --operation-id=pause-1234567890

# 强制恢复（忽略修改检查）
./bin/devbox-restore --backup-dir=./backup --force
```

### 恢复controller
```bash
# 恢复controller deployment
kubectl apply -f backup/controller_deployment.yaml

# 检查controller状态
kubectl get pods -n devbox-system
kubectl logs -n devbox-system deployment/devbox-controller-manager
```

### 恢复CRD
```bash
# 如果CRD更新失败，恢复原始CRD
kubectl apply -f backup/devboxes_crd.yaml
kubectl apply -f backup/devboxreleases_crd.yaml
```

## 场景7: 测试环境快速升级

```bash
# 跳过备份和暂停，直接转换
./scripts/upgrade-v1-to-v2.sh --skip-backup --skip-pause
```

## 场景8: 自定义controller信息

```bash
# 自定义controller命名空间和名称
./bin/devbox-pause --controller-namespace=my-system \
                   --controller-name=my-devbox-controller \
                   --backup-dir=./backup
```

## 场景9: 监控升级过程

```bash
# 在另一个终端监控资源状态
watch -n 2 'kubectl get devboxes -A && echo "---" && kubectl get devboxreleases -A'

# 监控CRD状态
watch -n 5 './bin/devbox-crd --action=check-status'

# 监控controller状态
watch -n 2 'kubectl get pods -n devbox-system'
```

## 场景10: 升级前检查

```bash
# 检查当前资源数量
echo "Devboxes: $(kubectl get devboxes -A --no-headers | wc -l)"
echo "DevboxReleases: $(kubectl get devboxreleases -A --no-headers | wc -l)"

# 检查CRD当前状态
./bin/devbox-crd --action=check-status

# 检查controller状态
kubectl get pods -n devbox-system

# 检查是否有正在进行的操作
kubectl get devboxes -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.state}{"\t"}{.status.phase}{"\n"}{end}'
```

## 常用命令组合

### 完整的生产升级流程
```bash
#!/bin/bash
set -e

BACKUP_DIR="./backup-$(date +%Y%m%d_%H%M%S)"

echo "Starting production upgrade..."

# 1. 预检查
./bin/devbox-crd --action=check-status
kubectl get devboxes -A --no-headers | wc -l

# 2. 备份
./bin/devbox-backup --backup-dir="$BACKUP_DIR"

# 3. 暂停
./bin/devbox-pause --backup-dir="$BACKUP_DIR"

# 4. 更新CRD
./bin/devbox-crd --action=apply --crd-dir=./scripts/manifests

# 5. 转换
./bin/devbox-transform --batch-size=5 --delay=1s

# 6. 最终化
./bin/devbox-crd --action=disable-v1alpha1

# 7. 验证
./bin/devbox-crd --action=check-status

echo "Upgrade completed! Backup stored in: $BACKUP_DIR"
```

### 快速回滚脚本
```bash
#!/bin/bash
set -e

BACKUP_DIR=${1:-"./backup"}

echo "Starting rollback from $BACKUP_DIR..."

# 1. 恢复devbox状态
./bin/devbox-restore --backup-dir="$BACKUP_DIR" --force

# 2. 恢复controller
if [ -f "$BACKUP_DIR/controller_deployment.yaml" ]; then
    kubectl apply -f "$BACKUP_DIR/controller_deployment.yaml"
fi

# 3. 检查状态
kubectl get devboxes -A
kubectl get pods -n devbox-system

echo "Rollback completed!"
```
