/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
)

// Upgrade related annotations
const (
	AnnotationUpgradeStatus    = "devbox.sealos.io/upgrade-status"
	AnnotationUpgradeStep      = "devbox.sealos.io/upgrade-step"
	AnnotationUpgradeOperation = "devbox.sealos.io/upgrade-operation-id"
	AnnotationUpgradeTimestamp = "devbox.sealos.io/upgrade-timestamp"
	AnnotationOriginalState    = "devbox.sealos.io/original-state"
	AnnotationUpgradeVersion   = "devbox.sealos.io/upgrade-version"
	AnnotationUpgradeError     = "devbox.sealos.io/upgrade-error"
	AnnotationUpgradeProgress  = "devbox.sealos.io/upgrade-progress"
)

// Upgrade status values
const (
	UpgradeStatusPending    = "pending"
	UpgradeStatusInProgress = "in-progress"
	UpgradeStatusPaused     = "paused"
	UpgradeStatusCompleted  = "completed"
	UpgradeStatusFailed     = "failed"
	UpgradeStatusRolledBack = "rolled-back"
)

// Upgrade steps
const (
	UpgradeStepBackup    = "backup"
	UpgradeStepPause     = "pause"
	UpgradeStepCRDUpdate = "crd-update"
	UpgradeStepTransform = "transform"
	UpgradeStepFinalize  = "finalize"
	UpgradeStepRestore   = "restore"
)

// UpgradeInfo contains upgrade information
type UpgradeInfo struct {
	OperationID   string
	Step          string
	Status        string
	Version       string
	OriginalState string
	Error         string
	Progress      string
}

// AddUpgradeAnnotations 添加升级相关的annotations
func AddUpgradeAnnotations(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox, info UpgradeInfo) error {
	// 获取最新版本
	latest := &devboxv1alpha1.Devbox{}
	key := types.NamespacedName{Name: devbox.Name, Namespace: devbox.Namespace}
	if err := k8sClient.Get(ctx, key, latest); err != nil {
		return fmt.Errorf("failed to get latest devbox: %w", err)
	}

	if latest.Annotations == nil {
		latest.Annotations = make(map[string]string)
	}

	// 记录原始状态（只在第一次设置）
	if info.OriginalState != "" {
		if _, exists := latest.Annotations[AnnotationOriginalState]; !exists {
			latest.Annotations[AnnotationOriginalState] = info.OriginalState
		}
	}

	// 设置升级相关annotations
	if info.OperationID != "" {
		latest.Annotations[AnnotationUpgradeOperation] = info.OperationID
	}
	if info.Step != "" {
		latest.Annotations[AnnotationUpgradeStep] = info.Step
	}
	if info.Status != "" {
		latest.Annotations[AnnotationUpgradeStatus] = info.Status
	}
	if info.Version != "" {
		latest.Annotations[AnnotationUpgradeVersion] = info.Version
	}
	if info.Error != "" {
		latest.Annotations[AnnotationUpgradeError] = info.Error
	}
	if info.Progress != "" {
		latest.Annotations[AnnotationUpgradeProgress] = info.Progress
	}

	latest.Annotations[AnnotationUpgradeTimestamp] = time.Now().Format(time.RFC3339)

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to update devbox annotations: %w", err)
	}

	return nil
}

// UpdateUpgradeAnnotation 更新单个升级annotation
func UpdateUpgradeAnnotation(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox, annotationKey, value string) error {
	// 获取最新版本
	latest := &devboxv1alpha1.Devbox{}
	key := types.NamespacedName{Name: devbox.Name, Namespace: devbox.Namespace}
	if err := k8sClient.Get(ctx, key, latest); err != nil {
		return fmt.Errorf("failed to get latest devbox: %w", err)
	}

	if latest.Annotations == nil {
		latest.Annotations = make(map[string]string)
	}

	latest.Annotations[annotationKey] = value
	latest.Annotations[AnnotationUpgradeTimestamp] = time.Now().Format(time.RFC3339)

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to update devbox annotation: %w", err)
	}

	return nil
}

// GetUpgradeInfo 获取devbox的升级信息
func GetUpgradeInfo(devbox *devboxv1alpha1.Devbox) UpgradeInfo {
	if devbox.Annotations == nil {
		return UpgradeInfo{}
	}

	return UpgradeInfo{
		OperationID:   devbox.Annotations[AnnotationUpgradeOperation],
		Step:          devbox.Annotations[AnnotationUpgradeStep],
		Status:        devbox.Annotations[AnnotationUpgradeStatus],
		Version:       devbox.Annotations[AnnotationUpgradeVersion],
		OriginalState: devbox.Annotations[AnnotationOriginalState],
		Error:         devbox.Annotations[AnnotationUpgradeError],
		Progress:      devbox.Annotations[AnnotationUpgradeProgress],
	}
}

// ClearUpgradeAnnotations 清理升级相关的annotations
func ClearUpgradeAnnotations(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox) error {
	// 获取最新版本
	latest := &devboxv1alpha1.Devbox{}
	key := types.NamespacedName{Name: devbox.Name, Namespace: devbox.Namespace}
	if err := k8sClient.Get(ctx, key, latest); err != nil {
		return fmt.Errorf("failed to get latest devbox: %w", err)
	}

	if latest.Annotations == nil {
		return nil
	}

	// 删除升级相关的annotations
	upgradeAnnotations := []string{
		AnnotationUpgradeStatus,
		AnnotationUpgradeStep,
		AnnotationUpgradeOperation,
		AnnotationUpgradeTimestamp,
		AnnotationUpgradeVersion,
		AnnotationUpgradeError,
		AnnotationUpgradeProgress,
		// 保留 AnnotationOriginalState，可能在回滚时需要
	}

	modified := false
	for _, annotation := range upgradeAnnotations {
		if _, exists := latest.Annotations[annotation]; exists {
			delete(latest.Annotations, annotation)
			modified = true
		}
	}

	if !modified {
		return nil
	}

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to clear upgrade annotations: %w", err)
	}

	return nil
}

// IsUpgradeInProgress 检查devbox是否正在升级中
func IsUpgradeInProgress(devbox *devboxv1alpha1.Devbox) bool {
	if devbox.Annotations == nil {
		return false
	}

	status, exists := devbox.Annotations[AnnotationUpgradeStatus]
	if !exists {
		return false
	}

	return status == UpgradeStatusInProgress || status == UpgradeStatusPending
}

// HasUpgradeFailed 检查devbox升级是否失败
func HasUpgradeFailed(devbox *devboxv1alpha1.Devbox) bool {
	if devbox.Annotations == nil {
		return false
	}

	status, exists := devbox.Annotations[AnnotationUpgradeStatus]
	return exists && status == UpgradeStatusFailed
}
