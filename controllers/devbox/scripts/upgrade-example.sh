#!/bin/bash

# Devbox v1alpha1 到 v1alpha2 升级示例脚本
# 这个脚本展示了如何使用 cmd/upgrade 工具进行完整的升级流程

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPGRADE_TOOL="${SCRIPT_DIR}/../cmd/upgrade/main.go"
BACKUP_DIR="${SCRIPT_DIR}/../backup-$(date +%Y%m%d-%H%M%S)"
CRD_DIR="${SCRIPT_DIR}/manifests"

echo "=== Devbox v1alpha1 -> v1alpha2 升级流程 ==="
echo "备份目录: $BACKUP_DIR"
echo "CRD目录: $CRD_DIR"
echo

# 检查工具是否存在
if [ ! -f "$UPGRADE_TOOL" ]; then
    echo "错误: 升级工具不存在: $UPGRADE_TOOL"
    exit 1
fi

# 检查CRD文件是否存在
if [ ! -f "$CRD_DIR/devbox_v1alpha2_crd.yaml" ]; then
    echo "错误: devbox CRD文件不存在: $CRD_DIR/devbox_v1alpha2_crd.yaml"
    exit 1
fi

if [ ! -f "$CRD_DIR/devboxrelease_v1alpha2_crd.yaml" ]; then
    echo "错误: devboxrelease CRD文件不存在: $CRD_DIR/devboxrelease_v1alpha2_crd.yaml"
    exit 1
fi

echo "1. 执行干运行以检查升级计划..."
go run "$UPGRADE_TOOL" \
    --dry-run \
    --backup-dir="$BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "2. 仅执行备份步骤..."
go run "$UPGRADE_TOOL" \
    --only-backup \
    --backup-dir="$BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "3. 仅暂停所有devbox..."
go run "$UPGRADE_TOOL" \
    --only-pause \
    --backup-dir="$BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "4. 仅执行CR转换..."
go run "$UPGRADE_TOOL" \
    --only-transform \
    --backup-dir="$BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "5. 执行完整升级流程..."
echo "注意: 这将执行所有剩余步骤（暂停controller、更新CRD、禁用v1alpha1）"
read -p "继续执行完整升级? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    go run "$UPGRADE_TOOL" \
        --backup-dir="$BACKUP_DIR" \
        --crd-dir="$CRD_DIR" \
        --skip-backup \
        --skip-pause
else
    echo "跳过完整升级"
fi

echo
echo "=== 升级流程完成 ==="
echo "备份文件位置: $BACKUP_DIR"
echo
echo "升级后验证:"
echo "1. 检查所有devbox是否正常运行"
echo "2. 检查CRD版本是否正确"
echo "3. 验证数据完整性"
echo
echo "回滚命令（如需要）:"
echo "kubectl apply -f $BACKUP_DIR/devboxes_crd.yaml"
echo "kubectl apply -f $BACKUP_DIR/devboxreleases_crd.yaml"
echo "kubectl apply -f $BACKUP_DIR/controller_deployment.yaml"
