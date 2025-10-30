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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
	"github.com/labring/sealos/controllers/devbox/pkg/upgrade"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("devbox-restore")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

// DevboxBackupState 记录devbox的原始状态，用于回滚
type DevboxBackupState struct {
	Name        string                     `json:"name"`
	Namespace   string                     `json:"namespace"`
	State       devboxv1alpha2.DevboxState `json:"state"`
	Phase       devboxv1alpha2.DevboxPhase `json:"phase"`
	OperationID string                     `json:"operationId"`
	BackupTime  string                     `json:"backupTime"`
}

type RestoreConfig struct {
	DryRun           bool
	BackupDir        string
	BackupStatesFile string
	OperationID      string
	OnlyStates       bool
	Force            bool
	BatchSize        int // 每次恢复的最大数量，0 表示不限制
}

func main() {
	var config RestoreConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory containing backup files")
	flag.StringVar(&config.BackupStatesFile, "backup-states", "", "Specific backup states file (default: backup-dir/devbox_backup_states.json)")
	flag.StringVar(&config.OperationID, "operation-id", "", "Specific operation ID to restore (empty for latest)")
	flag.BoolVar(&config.OnlyStates, "only-states", false, "Only restore devbox states, not full resources")
	flag.BoolVar(&config.Force, "force", false, "Force restore even if devbox has been modified")
	flag.IntVar(&config.BatchSize, "batch-size", 0, "Maximum number of devboxes to restore per run (0 = no limit)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	kubeConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}

	ctx := context.Background()

	// 默认备份状态文件路径
	if config.BackupStatesFile == "" {
		config.BackupStatesFile = filepath.Join(config.BackupDir, "devbox_backup_states.json")
	}

	setupLog.Info("Starting devbox restore process",
		"dry-run", config.DryRun,
		"backup-dir", config.BackupDir,
		"backup-states-file", config.BackupStatesFile,
		"operation-id", config.OperationID,
		"only-states", config.OnlyStates,
		"force", config.Force,
		"batch-size", config.BatchSize)

	if err := performRestore(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "restore process failed")
		os.Exit(1)
	}

	setupLog.Info("Restore process completed successfully")
}

func performRestore(ctx context.Context, k8sClient client.Client, config RestoreConfig) error {
	// 加载备份状态
	backupStates, err := loadBackupStates(config.BackupStatesFile)
	if err != nil {
		return fmt.Errorf("failed to load backup states: %w", err)
	}

	setupLog.Info("Loaded backup states", "count", len(backupStates))

	// 过滤指定的操作ID
	var targetStates []DevboxBackupState
	if config.OperationID != "" {
		for _, state := range backupStates {
			if state.OperationID == config.OperationID {
				targetStates = append(targetStates, state)
			}
		}
		if len(targetStates) == 0 {
			return fmt.Errorf("no backup states found for operation ID: %s", config.OperationID)
		}
	} else {
		targetStates = backupStates
	}

	// 筛选出需要恢复的 devbox（状态不匹配的）
	var devboxesToRestore []DevboxBackupState
	for _, state := range targetStates {
		needsRestore, err := needsRestoreCheck(ctx, k8sClient, state, config)
		if err != nil {
			setupLog.Error(err, "Failed to check if devbox needs restore",
				"name", state.Name,
				"namespace", state.Namespace)
			if !config.Force {
				return err
			}
			continue
		}
		if needsRestore {
			devboxesToRestore = append(devboxesToRestore, state)
		}
	}

	setupLog.Info("Found devboxes needing restore",
		"total", len(targetStates),
		"needs-restore", len(devboxesToRestore))

	// 应用批量限制
	restoreCount := len(devboxesToRestore)
	if config.BatchSize > 0 && restoreCount > config.BatchSize {
		restoreCount = config.BatchSize
		setupLog.Info("Applying batch size limit",
			"batch-size", config.BatchSize,
			"remaining", len(devboxesToRestore)-config.BatchSize)
	}

	if restoreCount == 0 {
		setupLog.Info("No devboxes need to be restored")
		return nil
	}

	// 恢复devbox状态（只处理批量限制内的数量）
	successCount := 0
	failCount := 0
	for i := 0; i < restoreCount; i++ {
		state := devboxesToRestore[i]
		if err := restoreDevboxState(ctx, k8sClient, state, config); err != nil {
			setupLog.Error(err, "Failed to restore devbox state",
				"name", state.Name,
				"namespace", state.Namespace)
			failCount++
			if !config.Force {
				return err
			}
		} else {
			successCount++
		}
	}

	setupLog.Info("Restore batch completed",
		"restored", successCount,
		"failed", failCount,
		"remaining", len(devboxesToRestore)-restoreCount)

	return nil
}

// needsRestoreCheck 检查 devbox 是否需要恢复（当前状态与备份状态不匹配）
func needsRestoreCheck(ctx context.Context, k8sClient client.Client, state DevboxBackupState, config RestoreConfig) (bool, error) {
	// 获取当前devbox
	devbox := &devboxv1alpha2.Devbox{}
	key := types.NamespacedName{Name: state.Name, Namespace: state.Namespace}
	if err := k8sClient.Get(ctx, key, devbox); err != nil {
		return false, fmt.Errorf("failed to get devbox %s/%s: %w", state.Namespace, state.Name, err)
	}

	// 检查状态是否匹配
	if devbox.Spec.State == state.State {
		setupLog.V(1).Info("Devbox state already matches backup, skipping",
			"name", state.Name,
			"namespace", state.Namespace,
			"state", state.State)
		return false, nil
	}

	return true, nil
}

func restoreDevboxState(ctx context.Context, k8sClient client.Client, state DevboxBackupState, config RestoreConfig) error {
	setupLog.Info("Restoring devbox state",
		"name", state.Name,
		"namespace", state.Namespace,
		"target-state", state.State,
		"operation-id", state.OperationID)

	if config.DryRun {
		setupLog.Info("DRY-RUN: Would restore devbox state",
			"name", state.Name,
			"namespace", state.Namespace,
			"state", state.State)
		return nil
	}

	// 获取当前devbox
	devbox := &devboxv1alpha2.Devbox{}
	key := types.NamespacedName{Name: state.Name, Namespace: state.Namespace}
	if err := k8sClient.Get(ctx, key, devbox); err != nil {
		return fmt.Errorf("failed to get devbox %s/%s: %w", state.Namespace, state.Name, err)
	}

	currentState := devbox.Spec.State

	// 检查是否被修改过（如果不是强制模式）
	if !config.Force {
		upgradeInfo := upgrade.GetUpgradeInfoV1Alpha2(devbox)
		if upgradeInfo.Status != "" {
			setupLog.Info("Devbox has upgrade status, may have been modified",
				"name", state.Name,
				"namespace", state.Namespace,
				"upgrade-status", upgradeInfo.Status,
				"upgrade-step", upgradeInfo.Step)
		}
	}

	// 添加恢复注解
	restoreInfo := upgrade.UpgradeInfo{
		OperationID: fmt.Sprintf("restore-%s", state.OperationID),
		Step:        upgrade.UpgradeStepRestore,
		Version:     "restore-v1alpha2",
	}
	if err := upgrade.AddUpgradeAnnotationsV1Alpha2(ctx, k8sClient, devbox, restoreInfo); err != nil {
		setupLog.Error(err, "Failed to add restore annotations")
	}

	// 重新获取最新版本
	if err := k8sClient.Get(ctx, key, devbox); err != nil {
		return fmt.Errorf("failed to get devbox %s/%s: %w", state.Namespace, state.Name, err)
	}

	// 恢复状态
	devbox.Spec.State = state.State

	if err := k8sClient.Update(ctx, devbox); err != nil {
		return fmt.Errorf("failed to restore devbox %s/%s: %w", state.Namespace, state.Name, err)
	}

	setupLog.Info("Successfully restored devbox state",
		"name", state.Name,
		"namespace", state.Namespace,
		"from-state", currentState,
		"to-state", state.State,
		"operation-id", state.OperationID)

	return nil
}

// loadBackupStates 从JSON文件加载备份状态
func loadBackupStates(filename string) ([]DevboxBackupState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup states file: %w", err)
	}

	var states []DevboxBackupState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup states: %w", err)
	}

	return states, nil
}
