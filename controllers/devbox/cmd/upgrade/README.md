# Devbox 升级工具

这个工具用于将 Devbox 从 v1alpha1 版本升级到 v1alpha2 版本。

## 升级流程

完整的升级流程包含以下步骤：

1. **备份CR & CRD** - 备份所有现有的自定义资源和自定义资源定义
2. **暂停所有devbox** - 停止所有运行中的devbox，等待commit操作完成，并记录原始状态
3. **暂停Controller** - 删除devbox controller deployment以避免冲突
4. **CRD更新** - 使用新的v1alpha2 CRD定义更新集群
5. **CR转换** - 强制转换所有现有的CR到新的存储版本
6. **最终CRD更新** - 禁用v1alpha1版本，从storedVersions中移除

## 使用方法

### 基本命令

```bash
# 完整升级流程
go run main.go --backup-dir ./backup --crd-dir ./scripts/manifests

# 干运行模式（仅显示将要执行的操作）
go run main.go --dry-run --backup-dir ./backup --crd-dir ./scripts/manifests

# 指定命名空间
go run main.go --namespace my-namespace --backup-dir ./backup --crd-dir ./scripts/manifests
```

### 分步执行

```bash
# 仅备份
go run main.go --only-backup --backup-dir ./backup --crd-dir ./scripts/manifests

# 仅暂停devbox
go run main.go --only-pause --backup-dir ./backup --crd-dir ./scripts/manifests

# 仅执行CR转换
go run main.go --only-transform --backup-dir ./backup --crd-dir ./scripts/manifests

# 跳过某些步骤
go run main.go --skip-backup --skip-pause --backup-dir ./backup --crd-dir ./scripts/manifests
```

## 参数说明

- `--dry-run`: 干运行模式，仅显示将要执行的操作，不实际修改集群
- `--namespace`: 指定要升级的命名空间（默认为所有命名空间）
- `--backup-dir`: 备份文件存储目录（默认为 `./backup`）
- `--crd-dir`: CRD文件所在目录（默认为 `./scripts/manifests`）
- `--skip-backup`: 跳过备份步骤
- `--skip-pause`: 跳过暂停devbox步骤
- `--only-backup`: 仅执行备份操作
- `--only-pause`: 仅执行暂停devbox操作
- `--only-transform`: 仅执行CR转换操作

## 备份文件

升级工具会在指定的备份目录中创建以下文件：

- `devboxes_crd.yaml`: Devboxes CRD的备份
- `devboxreleases_crd.yaml`: DevboxReleases CRD的备份
- `controller_deployment.yaml`: Controller Deployment的备份
- `devbox_<namespace>_<name>.yaml`: 每个Devbox的备份
- `devboxrelease_<namespace>_<name>.yaml`: 每个DevboxRelease的备份
- `devbox_backup_states.json`: Devbox原始状态记录（用于回滚）

## 回滚

如果升级过程中出现问题，可以使用备份文件进行回滚：

```bash
# 恢复CRD
kubectl apply -f backup/devboxes_crd.yaml
kubectl apply -f backup/devboxreleases_crd.yaml

# 恢复Controller
kubectl apply -f backup/controller_deployment.yaml

# 恢复Devbox状态（需要手动根据backup_states.json文件恢复）
```

## 前置条件

1. 确保集群中已安装Devbox v1alpha1版本
2. 确保`scripts/manifests`目录包含以下文件：
   - `devbox_v1alpha2_crd.yaml`
   - `devboxrelease_v1alpha2_crd.yaml`
3. 确保有足够的权限操作CRD和Deployment

## 注意事项

1. **升级前务必备份**: 虽然工具会自动备份，但建议在升级前手动备份重要数据
2. **停机时间**: 升级过程中devbox服务将暂时不可用
3. **权限要求**: 需要集群管理员权限来操作CRD和系统资源
4. **网络要求**: 确保能够访问Kubernetes API服务器
5. **存储要求**: 确保有足够的磁盘空间存储备份文件

## 故障排除

### 常见错误

1. **CRD文件不存在**: 检查`--crd-dir`参数指向的目录是否包含必要的CRD文件
2. **权限不足**: 确保当前用户有足够的权限操作集群资源
3. **Commit超时**: 如果devbox的commit操作长时间未完成，可能需要手动干预

### 日志级别

工具使用结构化日志，可以通过环境变量调整日志级别：

```bash
export LOG_LEVEL=debug
go run main.go --dry-run
```

## 验证升级

升级完成后，建议执行以下验证步骤：

1. 检查CRD版本：
   ```bash
   kubectl get crd devboxes.devbox.sealos.io -o yaml | grep -A5 versions
   kubectl get crd devboxreleases.devbox.sealos.io -o yaml | grep -A5 versions
   ```

2. 检查所有devbox状态：
   ```bash
   kubectl get devboxes --all-namespaces
   kubectl get devboxreleases --all-namespaces
   ```

3. 验证controller是否正常运行：
   ```bash
   kubectl get deployment -n devbox-system devbox-controller-manager
   ```
