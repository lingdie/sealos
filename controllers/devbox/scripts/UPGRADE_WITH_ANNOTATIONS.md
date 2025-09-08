# 带状态跟踪的Devbox升级系统

## 概述

我们已经成功实现了将升级状态记录到devbox annotations中的功能，使得整个升级过程更加透明和可控。

## 新增功能

### 1. 状态跟踪Annotations

每个devbox在升级过程中会自动添加以下annotations：

```yaml
metadata:
  annotations:
    devbox.sealos.io/upgrade-status: "paused"           # 升级状态
    devbox.sealos.io/upgrade-step: "pause"              # 当前步骤
    devbox.sealos.io/upgrade-operation-id: "pause-1757324658"  # 操作ID
    devbox.sealos.io/upgrade-timestamp: "2025-09-08T17:44:18+08:00"  # 时间戳
    devbox.sealos.io/original-state: "Stopped"          # 原始状态
    devbox.sealos.io/upgrade-version: "v1alpha1-to-v1alpha2"  # 升级版本
```

### 2. 新增工具

#### devbox-status工具
专门用于查看和监控devbox的升级状态：

```bash
# 查看所有devbox状态
./bin/devbox-status --all

# 只查看正在升级的devbox
./bin/devbox-status --only-upgrading

# 特定命名空间
./bin/devbox-status --namespace=production

# 不同输出格式
./bin/devbox-status --output=json
./bin/devbox-status --output=yaml
```

#### 公共annotations包
`pkg/upgrade/annotations.go` 提供了统一的annotation管理功能：

```go
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

// 获取升级信息
upgradeInfo := upgrade.GetUpgradeInfo(devbox)
```

## 升级流程状态跟踪

### 1. 暂停阶段（devbox-pause）

```
开始 -> in-progress (pause) -> paused/failed
```

- 记录原始状态
- 等待commits完成
- 设置为Stopped状态
- 标记为paused

### 2. 转换阶段（devbox-transform）

```
paused -> in-progress (transform) -> completed/failed
```

- 显示进度信息 (1/5, 2/5, ...)
- 强制存储版本转换
- 标记为completed

### 3. 恢复阶段（devbox-restore）

```
any -> rolled-back (restore)
```

- 恢复到原始状态
- 标记为rolled-back

## 实际使用示例

### 1. 执行升级并监控

```bash
# 终端1: 执行升级
./scripts/upgrade-v1-to-v2.sh

# 终端2: 监控进度
watch -n 2 './bin/devbox-status --only-upgrading'
```

### 2. 查看升级状态

```bash
# 表格格式查看
$ ./bin/devbox-status --all
NAMESPACE   NAME   CURRENT STATE   ORIGINAL STATE   UPGRADE STATUS   UPGRADE STEP   OPERATION ID   PROGRESS   TIMESTAMP
default   devbox-sample-1   Stopped   Stopped   paused   pause   pause-1757324658   -   2025-09-08T17:44:18+08:00
```

### 3. 检查具体annotations

```bash
# 使用kubectl查看
kubectl get devbox devbox-sample-1 -o jsonpath='{.metadata.annotations}' | jq

# 查看特定状态
kubectl get devbox devbox-sample-1 -o jsonpath='{.metadata.annotations.devbox\.sealos\.io/upgrade-status}'
```

### 4. 故障排除

```bash
# 找出失败的升级
./bin/devbox-status --all | grep failed

# 查看错误信息
kubectl get devbox <name> -o jsonpath='{.metadata.annotations.devbox\.sealos\.io/upgrade-error}'
```

## 工具集成

### 升级脚本增强

`scripts/upgrade-v1-to-v2.sh` 现在包含：

- 升级进度显示
- 状态检查功能
- 详细的CRD和devbox状态报告

### Makefile更新

```bash
# 构建所有工具（包括devbox-status）
make build-upgrade-tools

# 单独构建status工具
make build-status
```

## 优势

### 1. 可观测性
- 实时查看每个devbox的升级状态
- 详细的进度信息和时间戳
- 支持多种输出格式

### 2. 故障排除
- 记录错误信息到annotations
- 保留原始状态用于回滚
- 操作ID用于跟踪相关操作

### 3. 批量管理
- 使用kubectl和jq进行批量查询
- 支持按状态过滤devbox
- 便于自动化脚本集成

### 4. 安全性
- 所有annotation操作都是并发安全的
- 失败时不会影响主要升级流程
- 支持干运行模式测试

## 向后兼容

- 旧版本的devbox没有这些annotations是正常的
- status工具会显示"-"表示没有升级信息
- 不会影响现有的devbox功能

## 性能影响

- Annotations增加的存储开销很小
- 每次操作会额外进行1-2次API调用
- 对整体升级性能影响微乎其微

## 扩展性

这个annotation系统为未来的功能提供了基础：

- 支持其他版本的升级跟踪
- 可以添加更多的状态和步骤
- 便于集成监控和告警系统
- 支持自定义升级流程

通过这个系统，Devbox升级过程变得更加透明、可控和可靠。管理员可以随时了解升级状态，快速定位问题，并在必要时进行干预。
