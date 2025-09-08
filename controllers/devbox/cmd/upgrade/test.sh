#!/bin/bash

# 简单的升级工具测试脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPGRADE_TOOL="$SCRIPT_DIR/main.go"
TEST_BACKUP_DIR="$SCRIPT_DIR/test-backup"
CRD_DIR="$SCRIPT_DIR/../../scripts/manifests"

echo "=== 测试 Devbox 升级工具 ==="
echo

# 清理之前的测试文件
if [ -d "$TEST_BACKUP_DIR" ]; then
    echo "清理之前的测试备份目录..."
    rm -rf "$TEST_BACKUP_DIR"
fi

echo "1. 测试干运行模式..."
go run "$UPGRADE_TOOL" \
    --dry-run \
    --backup-dir="$TEST_BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "2. 测试仅备份模式..."
go run "$UPGRADE_TOOL" \
    --only-backup \
    --backup-dir="$TEST_BACKUP_DIR" \
    --crd-dir="$CRD_DIR"

echo
echo "3. 检查备份文件是否创建..."
if [ -d "$TEST_BACKUP_DIR" ]; then
    echo "✓ 备份目录已创建: $TEST_BACKUP_DIR"
    echo "备份文件列表:"
    ls -la "$TEST_BACKUP_DIR/"
else
    echo "✗ 备份目录未创建"
    exit 1
fi

echo
echo "4. 测试帮助信息..."
go run "$UPGRADE_TOOL" --help || true

echo
echo "=== 测试完成 ==="
echo "测试备份目录: $TEST_BACKUP_DIR"
echo
echo "清理测试文件? (y/N):"
read -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf "$TEST_BACKUP_DIR"
    echo "测试文件已清理"
fi
