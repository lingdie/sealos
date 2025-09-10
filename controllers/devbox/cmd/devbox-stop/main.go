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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
	"github.com/labring/sealos/controllers/devbox/pkg/upgrade"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("devbox-stop")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

// DevboxBackupState 记录devbox的原始状态，用于回滚
type DevboxBackupState struct {
	Name        string                     `json:"name"`
	Namespace   string                     `json:"namespace"`
	State       devboxv1alpha1.DevboxState `json:"state"`
	Phase       devboxv1alpha1.DevboxPhase `json:"phase"`
	OperationID string                     `json:"operationId"`
	BackupTime  time.Time                  `json:"backupTime"`
}

type DevboxStopConfig struct {
	DryRun        bool
	BackupDir     string
	Namespace     string
	CommitTimeout time.Duration
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	BaseDelay:  100 * time.Millisecond,
	MaxDelay:   5 * time.Second,
}

// retryWithBackoff 执行带指数退避的重试操作
func retryWithBackoff(ctx context.Context, operation func() error, config RetryConfig, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算退避延迟
			delay := config.BaseDelay * time.Duration(1<<uint(attempt-1))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			setupLog.Info("Retrying operation",
				"operation", operationName,
				"attempt", attempt+1,
				"delay", delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				setupLog.Info("Operation succeeded after retry",
					"operation", operationName,
					"attempts", attempt+1)
			}
			return nil
		}

		lastErr = err
		setupLog.Error(err, "Operation failed",
			"operation", operationName,
			"attempt", attempt+1)
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operationName, config.MaxRetries+1, lastErr)
}

func main() {
	var config DevboxStopConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to stop devboxes (empty for all namespaces)")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")
	flag.DurationVar(&config.CommitTimeout, "commit-timeout", 5*time.Minute, "Timeout for waiting commits to finish")

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

	setupLog.Info("Starting devbox stop process",
		"dry-run", config.DryRun,
		"namespace", config.Namespace,
		"backup-dir", config.BackupDir,
		"commit-timeout", config.CommitTimeout)

	if err := stopAllDevboxes(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "devbox stop process failed")
		os.Exit(1)
	}

	setupLog.Info("Devbox stop process completed successfully")
}

// stopAllDevboxes 停止所有devbox，等待commit结束（记录操作id及原状态）
func stopAllDevboxes(ctx context.Context, k8sClient client.Client, config DevboxStopConfig) error {
	// 创建备份目录
	if !config.DryRun {
		if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Devboxes: %w", err)
	}

	setupLog.Info("Found Devboxes to stop", "count", len(devboxList.Items))

	var backupStates []DevboxBackupState
	operationID := fmt.Sprintf("stop-%d", time.Now().Unix())

	for i := range devboxList.Items {
		devbox := &devboxList.Items[i]

		// 记录原始状态
		backupState := DevboxBackupState{
			Name:        devbox.Name,
			Namespace:   devbox.Namespace,
			State:       devbox.Spec.State,
			Phase:       devbox.Status.Phase,
			OperationID: operationID,
			BackupTime:  time.Now(),
		}
		backupStates = append(backupStates, backupState)

		setupLog.Info("Processing Devbox for stop",
			"name", devbox.Name,
			"namespace", devbox.Namespace,
			"current-state", devbox.Spec.State,
			"current-phase", devbox.Status.Phase)

		if config.DryRun {
			setupLog.Info("DRY-RUN: Would stop Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace)
			continue
		}

		// 添加升级开始的annotation
		upgradeInfo := upgrade.UpgradeInfo{
			OperationID:   operationID,
			Step:          upgrade.UpgradeStepStop,
			Status:        upgrade.UpgradeStatusInProgress,
			Version:       "v1alpha1-to-v1alpha2",
			OriginalState: string(devbox.Spec.State),
		}
		if err := upgrade.AddUpgradeAnnotations(ctx, k8sClient, devbox, upgradeInfo); err != nil {
			setupLog.Error(err, "Failed to add upgrade annotations",
				"name", devbox.Name,
				"namespace", devbox.Namespace)
			return err
		}

		// 重新get devbox
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: devbox.Name, Namespace: devbox.Namespace}, devbox); err != nil {
			return fmt.Errorf("failed to get devbox: %w", err)
		}

		// 如果devbox正在运行，需要停止它
		if devbox.Spec.State == devboxv1alpha1.DevboxStateRunning {
			// 设置为Stopped状态
			devbox.Spec.State = devboxv1alpha1.DevboxStateStopped

			// 使用重试逻辑更新devbox状态
			updateOperation := func() error {
				return k8sClient.Update(ctx, devbox)
			}

			if err := retryWithBackoff(ctx, updateOperation, DefaultRetryConfig, fmt.Sprintf("update devbox %s/%s state", devbox.Namespace, devbox.Name)); err != nil {
				// 标记为失败
				failAnnotationOperation := func() error {
					return upgrade.UpdateUpgradeAnnotationv1alpha1(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusFailed)
				}
				if updateErr := retryWithBackoff(ctx, failAnnotationOperation, DefaultRetryConfig, fmt.Sprintf("update upgrade status annotation to failed for %s/%s", devbox.Namespace, devbox.Name)); updateErr != nil {
					setupLog.Error(updateErr, "Failed to update upgrade status annotation")
				}
				return fmt.Errorf("failed to stop devbox %s/%s: %w", devbox.Namespace, devbox.Name, err)
			}

			// 标记停止完成
			successAnnotationOperation := func() error {
				return upgrade.UpdateUpgradeAnnotationv1alpha1(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusStopped)
			}
			if err := retryWithBackoff(ctx, successAnnotationOperation, DefaultRetryConfig, fmt.Sprintf("update upgrade status annotation to stopped for %s/%s", devbox.Namespace, devbox.Name)); err != nil {
				setupLog.Error(err, "Failed to update upgrade status annotation")
			}

			setupLog.Info("Successfully stopped Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace,
				"operation-id", operationID)
		} else {
			// 如果不需要停止，直接标记为已处理
			noStopAnnotationOperation := func() error {
				return upgrade.UpdateUpgradeAnnotationv1alpha1(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusStopped)
			}
			if err := retryWithBackoff(ctx, noStopAnnotationOperation, DefaultRetryConfig, fmt.Sprintf("update upgrade status annotation to stopped (no stop needed) for %s/%s", devbox.Namespace, devbox.Name)); err != nil {
				setupLog.Error(err, "Failed to update upgrade status annotation")
			}
		}

		// 添加延迟避免过载API server
		time.Sleep(100 * time.Millisecond)
	}

	// 保存备份状态到文件
	if !config.DryRun {
		backupStatesFile := filepath.Join(config.BackupDir, "devbox_backup_states.json")
		if err := saveBackupStates(backupStates, backupStatesFile); err != nil {
			return fmt.Errorf("failed to save backup states: %w", err)
		}
		setupLog.Info("Saved devbox backup states", "file", backupStatesFile, "operation-id", operationID)
	}

	return nil
}

// saveBackupStates 保存备份状态到JSON文件
func saveBackupStates(states []DevboxBackupState, filename string) error {
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup states: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup states file: %w", err)
	}

	return nil
}
