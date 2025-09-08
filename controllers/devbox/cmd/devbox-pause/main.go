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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	sigsyaml "sigs.k8s.io/yaml"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
	"github.com/labring/sealos/controllers/devbox/pkg/upgrade"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("devbox-pause")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

// DevboxBackupState 记录devbox的原始状态，用于回滚
type DevboxBackupState struct {
	Name      string                     `json:"name"`
	Namespace string                     `json:"namespace"`
	State     devboxv1alpha1.DevboxState `json:"state"`
	Phase     devboxv1alpha1.DevboxPhase `json:"phase"`
	// 记录操作id，用于跟踪
	OperationID string    `json:"operationId"`
	BackupTime  time.Time `json:"backupTime"`
}

type PauseConfig struct {
	DryRun          bool
	Namespace       string
	BackupDir       string
	PauseController bool
	ControllerNS    string
	ControllerName  string
	CommitTimeout   time.Duration
	OnlyDevboxes    bool
	OnlyController  bool
}

func main() {
	var config PauseConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to pause devboxes (empty for all namespaces)")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")
	flag.BoolVar(&config.PauseController, "pause-controller", true, "Whether to pause the controller")
	flag.StringVar(&config.ControllerNS, "controller-namespace", "devbox-system", "Controller namespace")
	flag.StringVar(&config.ControllerName, "controller-name", "devbox-controller-manager", "Controller deployment name")
	flag.DurationVar(&config.CommitTimeout, "commit-timeout", 5*time.Minute, "Timeout for waiting commits to finish")
	flag.BoolVar(&config.OnlyDevboxes, "only-devboxes", false, "Only pause devboxes, not the controller")
	flag.BoolVar(&config.OnlyController, "only-controller", false, "Only pause the controller, not devboxes")

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

	setupLog.Info("Starting devbox pause process",
		"dry-run", config.DryRun,
		"namespace", config.Namespace,
		"backup-dir", config.BackupDir,
		"pause-controller", config.PauseController,
		"only-devboxes", config.OnlyDevboxes,
		"only-controller", config.OnlyController)

	if err := performPause(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "pause process failed")
		os.Exit(1)
	}

	setupLog.Info("Pause process completed successfully")
}

func performPause(ctx context.Context, k8sClient client.Client, config PauseConfig) error {
	// 创建备份目录
	if !config.DryRun {
		if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

	// 暂停devboxes
	if !config.OnlyController {
		if err := pauseAllDevboxes(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to pause devboxes: %w", err)
		}
	}

	// 暂停controller
	if config.PauseController && !config.OnlyDevboxes {
		if err := pauseController(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to pause controller: %w", err)
		}
	}

	return nil
}

// pauseAllDevboxes 暂停所有devbox，等待commit结束（记录操作id及原状态）
func pauseAllDevboxes(ctx context.Context, k8sClient client.Client, config PauseConfig) error {
	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Devboxes: %w", err)
	}

	setupLog.Info("Found Devboxes to pause", "count", len(devboxList.Items))

	var backupStates []DevboxBackupState
	operationID := fmt.Sprintf("pause-%d", time.Now().Unix())

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

		setupLog.Info("Processing Devbox for pause",
			"name", devbox.Name,
			"namespace", devbox.Namespace,
			"current-state", devbox.Spec.State,
			"current-phase", devbox.Status.Phase)

		if config.DryRun {
			setupLog.Info("DRY-RUN: Would pause Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace)
			continue
		}

		// 添加升级开始的annotation
		upgradeInfo := upgrade.UpgradeInfo{
			OperationID:   operationID,
			Step:          upgrade.UpgradeStepPause,
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

		// 如果devbox正在运行，需要暂停它
		if devbox.Spec.State == devboxv1alpha1.DevboxStateRunning {
			// 等待commit结束
			if err := waitForCommitsToFinish(ctx, k8sClient, devbox, config.CommitTimeout); err != nil {
				setupLog.Error(err, "Failed to wait for commits to finish",
					"name", devbox.Name,
					"namespace", devbox.Namespace)
				// 标记为失败
				if updateErr := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusFailed); updateErr != nil {
					setupLog.Error(updateErr, "Failed to update upgrade status annotation")
				}
				return err
			}

			// 设置为Stopped状态
			devbox.Spec.State = devboxv1alpha1.DevboxStateStopped
			if err := k8sClient.Update(ctx, devbox); err != nil {
				// 标记为失败
				if updateErr := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusFailed); updateErr != nil {
					setupLog.Error(updateErr, "Failed to update upgrade status annotation")
				}
				return fmt.Errorf("failed to pause devbox %s/%s: %w", devbox.Namespace, devbox.Name, err)
			}

			// 标记暂停完成
			if err := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusPaused); err != nil {
				setupLog.Error(err, "Failed to update upgrade status annotation")
			}

			setupLog.Info("Successfully paused Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace,
				"operation-id", operationID)
		} else {
			// 如果不需要暂停，直接标记为已处理
			if err := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusPaused); err != nil {
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

// waitForCommitsToFinish 等待所有commit操作完成
func waitForCommitsToFinish(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox, timeout time.Duration) error {
	setupLog.Info("Waiting for commits to finish", "devbox", devbox.Name, "namespace", devbox.Namespace)

	interval := 10 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 重新获取最新状态
		latest := &devboxv1alpha1.Devbox{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: devbox.Name, Namespace: devbox.Namespace}, latest); err != nil {
			return fmt.Errorf("failed to get latest devbox state: %w", err)
		}

		// 检查是否有正在进行的commit
		hasActiveCommits := false
		for _, commit := range latest.Status.CommitHistory {
			if commit.Status == devboxv1alpha1.CommitStatusPending || commit.PredicatedStatus == devboxv1alpha1.CommitStatusPending {
				hasActiveCommits = true
				break
			}
		}

		if !hasActiveCommits {
			setupLog.Info("All commits finished", "devbox", devbox.Name, "namespace", devbox.Namespace)
			return nil
		}

		setupLog.Info("Still waiting for commits to finish", "devbox", devbox.Name, "namespace", devbox.Namespace)
		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for commits to finish for devbox %s/%s", devbox.Namespace, devbox.Name)
}

// pauseController 暂停Controller（直接删除旧的devbox deployment）
func pauseController(ctx context.Context, k8sClient client.Client, config PauseConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would delete controller deployment",
			"deployment", config.ControllerName,
			"namespace", config.ControllerNS)
		return nil
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			setupLog.Info("Controller deployment not found, might be already deleted",
				"deployment", config.ControllerName,
				"namespace", config.ControllerNS)
			return nil
		}
		return fmt.Errorf("failed to get controller deployment: %w", err)
	}

	// 备份deployment配置
	backupFile := filepath.Join(config.BackupDir, "controller_deployment.yaml")
	if err := saveToFile(deployment, backupFile); err != nil {
		return fmt.Errorf("failed to backup controller deployment: %w", err)
	}

	// 删除deployment
	if err := k8sClient.Delete(ctx, deployment); err != nil {
		return fmt.Errorf("failed to delete controller deployment: %w", err)
	}

	setupLog.Info("Successfully paused controller by deleting deployment",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS,
		"backup-file", backupFile)

	return nil
}

// saveToFile 将对象保存为YAML文件
func saveToFile(obj client.Object, filename string) error {
	// 清理对象的运行时信息
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetGeneration(0)
	obj.SetManagedFields(nil)

	// 转换为YAML
	yamlData, err := sigsyaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
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
