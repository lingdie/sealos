# Devbox升级Annotations说明

为了更好地跟踪和管理Devbox的升级状态，我们在每个devbox资源的annotations中记录了详细的升级信息。

## Annotations定义

### 升级状态相关

| Annotation | 描述 | 示例值 |
|------------|------|--------|
| `devbox.sealos.io/upgrade-status` | 升级状态 | `pending`, `in-progress`, `paused`, `completed`, `failed`, `rolled-back` |
| `devbox.sealos.io/upgrade-step` | 当前升级步骤 | `backup`, `pause`, `crd-update`, `transform`, `finalize`, `restore` |
| `devbox.sealos.io/upgrade-operation-id` | 操作ID | `pause-1694172123`, `transform-1694172456` |
| `devbox.sealos.io/upgrade-timestamp` | 最后更新时间 | `2024-09-08T09:35:23Z` |
| `devbox.sealos.io/original-state` | 原始状态 | `Running`, `Stopped` |
| `devbox.sealos.io/upgrade-version` | 升级版本 | `v1alpha1-to-v1alpha2` |
| `devbox.sealos.io/upgrade-error` | 错误信息 | 升级失败时的错误描述 |
| `devbox.sealos.io/upgrade-progress` | 进度信息 | `5/10`, `completed` |

## 升级状态值

### Status状态
- `pending`: 等待开始升级
- `in-progress`: 正在升级中
- `paused`: 已暂停（等待下一步）
- `completed`: 升级完成
- `failed`: 升级失败
- `rolled-back`: 已回滚

### Step步骤
- `backup`: 备份阶段
- `pause`: 暂停devbox阶段
- `crd-update`: CRD更新阶段
- `transform`: CR转换阶段
- `finalize`: 最终化阶段
- `restore`: 恢复阶段

## 使用场景

### 1. 查看升级状态

```bash
# 查看所有devbox的升级状态
./bin/devbox-status --all

# 只查看正在升级的devbox
./bin/devbox-status --only-upgrading

# 查看特定namespace的状态
./bin/devbox-status --namespace=production

# 查看特定devbox的状态
./bin/devbox-status --devbox=my-devbox
```

### 2. 监控升级进度

```bash
# 表格格式
./bin/devbox-status --only-upgrading

# JSON格式
./bin/devbox-status --only-upgrading --output=json

# YAML格式
./bin/devbox-status --only-upgrading --output=yaml
```

### 3. 手动检查annotations

```bash
# 使用kubectl查看annotations
kubectl get devbox my-devbox -o jsonpath='{.metadata.annotations}' | jq

# 查看特定annotation
kubectl get devbox my-devbox -o jsonpath='{.metadata.annotations.devbox\.sealos\.io/upgrade-status}'
```

## 升级流程中的Annotations变化

### 1. 暂停阶段（devbox-pause）

```yaml
annotations:
  devbox.sealos.io/upgrade-status: "in-progress"
  devbox.sealos.io/upgrade-step: "pause"
  devbox.sealos.io/upgrade-operation-id: "pause-1694172123"
  devbox.sealos.io/upgrade-timestamp: "2024-09-08T09:35:23Z"
  devbox.sealos.io/original-state: "Running"
  devbox.sealos.io/upgrade-version: "v1alpha1-to-v1alpha2"
```

暂停完成后：
```yaml
annotations:
  devbox.sealos.io/upgrade-status: "paused"
  # 其他annotations保持不变，timestamp更新
```

### 2. 转换阶段（devbox-transform）

```yaml
annotations:
  devbox.sealos.io/upgrade-status: "in-progress"
  devbox.sealos.io/upgrade-step: "transform"
  devbox.sealos.io/upgrade-operation-id: "transform-1694172456"
  devbox.sealos.io/upgrade-progress: "5/10"
  # ... 其他annotations
```

转换完成后：
```yaml
annotations:
  devbox.sealos.io/upgrade-status: "completed"
  # 其他annotations保持不变
```

### 3. 失败情况

```yaml
annotations:
  devbox.sealos.io/upgrade-status: "failed"
  devbox.sealos.io/upgrade-step: "transform"
  devbox.sealos.io/upgrade-error: "timeout waiting for commits to finish"
  # ... 其他annotations
```

### 4. 恢复阶段（devbox-restore）

```yaml
annotations:
  devbox.sealos.io/upgrade-status: "rolled-back"
  devbox.sealos.io/upgrade-step: "restore"
  devbox.sealos.io/upgrade-operation-id: "restore-pause-1694172123"
  devbox.sealos.io/upgrade-version: "restore-v1alpha1"
  # 保留原始状态信息
```

## 编程接口

### Go代码中使用

```go
import "github.com/labring/sealos/controllers/devbox/pkg/upgrade"

// 获取升级信息
upgradeInfo := upgrade.GetUpgradeInfo(devbox)
fmt.Printf("Status: %s, Step: %s\n", upgradeInfo.Status, upgradeInfo.Step)

// 添加升级annotations
upgradeInfo := upgrade.UpgradeInfo{
    OperationID: "my-operation-123",
    Step:        upgrade.UpgradeStepTransform,
    Status:      upgrade.UpgradeStatusInProgress,
    Version:     "v1alpha1-to-v1alpha2",
    Progress:    "3/10",
}
err := upgrade.AddUpgradeAnnotations(ctx, k8sClient, devbox, upgradeInfo)

// 更新单个annotation
err := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox,
    upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusCompleted)

// 检查是否正在升级
if upgrade.IsUpgradeInProgress(devbox) {
    fmt.Println("Devbox is upgrading")
}

// 清理升级annotations
err := upgrade.ClearUpgradeAnnotations(ctx, k8sClient, devbox)
```

## 最佳实践

### 1. 监控升级进度

在升级过程中，定期检查升级状态：

```bash
# 在另一个终端监控升级进度
watch -n 5 './bin/devbox-status --only-upgrading'
```

### 2. 故障排除

当升级失败时：

```bash
# 查看失败的devbox
./bin/devbox-status --only-upgrading | grep failed

# 查看具体错误信息
kubectl get devbox <name> -o jsonpath='{.metadata.annotations.devbox\.sealos\.io/upgrade-error}'
```

### 3. 批量操作

使用annotations进行批量操作：

```bash
# 找出所有正在升级的devbox
kubectl get devbox -A -o jsonpath='{range .items[?(@.metadata.annotations.devbox\.sealos\.io/upgrade-status=="in-progress")]}{.metadata.namespace}{"\t"}{.metadata.name}{"\n"}{end}'

# 找出所有升级失败的devbox
kubectl get devbox -A -o jsonpath='{range .items[?(@.metadata.annotations.devbox\.sealos\.io/upgrade-status=="failed")]}{.metadata.namespace}{"\t"}{.metadata.name}{"\n"}{end}'
```

### 4. 清理annotations

升级完成后，可以选择清理升级相关的annotations：

```bash
# 使用restore工具清理（保留original-state）
./bin/devbox-restore --clear-annotations

# 或手动清理
kubectl annotate devbox <name> devbox.sealos.io/upgrade-status-
kubectl annotate devbox <name> devbox.sealos.io/upgrade-step-
# ... 其他annotations
```

## 注意事项

1. **并发安全**: 所有annotation操作都会先获取最新版本的devbox，确保并发安全
2. **时间戳**: 每次更新annotation时都会更新timestamp
3. **错误处理**: annotation操作失败不会中断主要的升级流程
4. **向后兼容**: 旧版本的devbox没有这些annotations是正常的
5. **存储开销**: annotations会增加一些存储开销，但相比于完整的devbox对象来说很小

通过这些annotations，我们可以精确地跟踪每个devbox的升级状态，提供更好的可观测性和故障排除能力。
