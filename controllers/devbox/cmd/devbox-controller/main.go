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
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("devbox-controller")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

type ControllerConfig struct {
	DryRun         bool
	Action         string // "pause", "resume", "status", "restart"
	ControllerNS   string
	ControllerName string
	BackupDir      string
	WaitTimeout    time.Duration
}

func main() {
	var config ControllerConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Action, "action", "status", "Action to perform: pause, resume, status, restart")
	flag.StringVar(&config.ControllerNS, "controller-namespace", "devbox-system", "Controller namespace")
	flag.StringVar(&config.ControllerName, "controller-name", "devbox-controller-manager", "Controller deployment name")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")
	flag.DurationVar(&config.WaitTimeout, "wait-timeout", 2*time.Minute, "Timeout for waiting controller operations")

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

	setupLog.Info("Starting devbox controller management",
		"dry-run", config.DryRun,
		"action", config.Action,
		"controller-namespace", config.ControllerNS,
		"controller-name", config.ControllerName)

	if err := performControllerAction(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "controller action failed")
		os.Exit(1)
	}

	setupLog.Info("Controller action completed successfully")
}

func performControllerAction(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	switch config.Action {
	case "pause":
		return pauseController(ctx, k8sClient, config)
	case "resume":
		return resumeController(ctx, k8sClient, config)
	case "status":
		return checkControllerStatus(ctx, k8sClient, config)
	case "restart":
		return restartController(ctx, k8sClient, config)
	default:
		return fmt.Errorf("unknown action: %s", config.Action)
	}
}

// pauseController 暂停Controller（删除deployment并备份）
func pauseController(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Pausing controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS)

	if config.DryRun {
		setupLog.Info("DRY-RUN: Would pause controller",
			"deployment", config.ControllerName,
			"namespace", config.ControllerNS)
		return nil
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			setupLog.Info("Controller deployment not found, might be already paused",
				"deployment", config.ControllerName,
				"namespace", config.ControllerNS)
			return nil
		}
		return fmt.Errorf("failed to get controller deployment: %w", err)
	}

	// 创建备份目录
	if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 备份deployment配置
	backupFile := filepath.Join(config.BackupDir, fmt.Sprintf("controller_deployment_%d.yaml", time.Now().Unix()))
	if err := saveToFile(deployment, backupFile); err != nil {
		return fmt.Errorf("failed to backup controller deployment: %w", err)
	}

	// 删除deployment
	if err := k8sClient.Delete(ctx, deployment); err != nil {
		return fmt.Errorf("failed to delete controller deployment: %w", err)
	}

	// 等待pods终止
	if err := waitForPodsTermination(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "Failed to wait for pods termination, but controller is paused")
	}

	setupLog.Info("Successfully paused controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS,
		"backup-file", backupFile)

	return nil
}

// resumeController 恢复Controller（从备份恢复deployment）
func resumeController(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Resuming controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS)

	if config.DryRun {
		setupLog.Info("DRY-RUN: Would resume controller",
			"deployment", config.ControllerName,
			"namespace", config.ControllerNS)
		return nil
	}

	// 检查是否已经存在
	existing := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, existing)
	if err == nil {
		setupLog.Info("Controller deployment already exists",
			"deployment", config.ControllerName,
			"namespace", config.ControllerNS,
			"replicas", *existing.Spec.Replicas)
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing deployment: %w", err)
	}

	// 查找最新的备份文件
	backupFile, err := findLatestBackupFile(config.BackupDir, "controller_deployment_*.yaml")
	if err != nil {
		return fmt.Errorf("failed to find backup file: %w", err)
	}

	setupLog.Info("Found backup file", "file", backupFile)

	// 从备份恢复deployment
	deployment, err := loadDeploymentFromFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to load deployment from backup: %w", err)
	}

	// 清理运行时信息
	deployment.ResourceVersion = ""
	deployment.UID = ""
	deployment.Generation = 0
	deployment.Status = appsv1.DeploymentStatus{}

	// 创建deployment
	if err := k8sClient.Create(ctx, deployment); err != nil {
		return fmt.Errorf("failed to create controller deployment: %w", err)
	}

	// 等待deployment就绪
	if err := waitForDeploymentReady(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "Controller deployment created but not ready yet")
	}

	setupLog.Info("Successfully resumed controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS,
		"backup-file", backupFile)

	return nil
}

// checkControllerStatus 检查Controller状态
func checkControllerStatus(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Checking controller status",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS)

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("Controller Status: PAUSED (deployment not found)\n")
			fmt.Printf("Deployment: %s/%s\n", config.ControllerNS, config.ControllerName)
			return nil
		}
		return fmt.Errorf("failed to get controller deployment: %w", err)
	}

	// 显示详细状态
	fmt.Printf("Controller Status: RUNNING\n")
	fmt.Printf("Deployment: %s/%s\n", config.ControllerNS, config.ControllerName)
	fmt.Printf("Desired Replicas: %d\n", *deployment.Spec.Replicas)
	fmt.Printf("Ready Replicas: %d\n", deployment.Status.ReadyReplicas)
	fmt.Printf("Available Replicas: %d\n", deployment.Status.AvailableReplicas)
	fmt.Printf("Updated Replicas: %d\n", deployment.Status.UpdatedReplicas)

	// 显示条件
	fmt.Printf("\nConditions:\n")
	for _, condition := range deployment.Status.Conditions {
		fmt.Printf("  %s: %s (%s)\n", condition.Type, condition.Status, condition.Reason)
		if condition.Message != "" {
			fmt.Printf("    Message: %s\n", condition.Message)
		}
	}

	return nil
}

// restartController 重启Controller
func restartController(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Restarting controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS)

	if config.DryRun {
		setupLog.Info("DRY-RUN: Would restart controller",
			"deployment", config.ControllerName,
			"namespace", config.ControllerNS)
		return nil
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, deployment)
	if err != nil {
		return fmt.Errorf("failed to get controller deployment: %w", err)
	}

	// 更新deployment的annotation来触发重启
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["devbox.sealos.io/restart-timestamp"] = fmt.Sprintf("%d", time.Now().Unix())

	if err := k8sClient.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update controller deployment: %w", err)
	}

	// 等待重启完成
	if err := waitForDeploymentReady(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "Controller restart initiated but not ready yet")
	}

	setupLog.Info("Successfully restarted controller",
		"deployment", config.ControllerName,
		"namespace", config.ControllerNS)

	return nil
}

// 辅助函数

func waitForPodsTermination(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Waiting for controller pods to terminate")

	deadline := time.Now().Add(config.WaitTimeout)
	for time.Now().Before(deadline) {
		// 这里可以添加检查pods的逻辑
		// 为了简化，我们只等待一小段时间
		time.Sleep(5 * time.Second)
		setupLog.Info("Controller pods should be terminating...")
		break
	}

	return nil
}

func waitForDeploymentReady(ctx context.Context, k8sClient client.Client, config ControllerConfig) error {
	setupLog.Info("Waiting for controller deployment to be ready")

	deadline := time.Now().Add(config.WaitTimeout)
	for time.Now().Before(deadline) {
		deployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: config.ControllerName, Namespace: config.ControllerNS}, deployment)
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			setupLog.Info("Controller deployment is ready",
				"ready-replicas", deployment.Status.ReadyReplicas,
				"desired-replicas", *deployment.Spec.Replicas)
			return nil
		}

		setupLog.Info("Waiting for deployment to be ready",
			"ready-replicas", deployment.Status.ReadyReplicas,
			"desired-replicas", *deployment.Spec.Replicas)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for controller deployment to be ready")
}

func findLatestBackupFile(backupDir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(backupDir, pattern))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no backup files found matching pattern: %s", pattern)
	}

	// 返回最新的文件（按名称排序，因为文件名包含时间戳）
	latest := matches[len(matches)-1]
	for _, match := range matches {
		if match > latest {
			latest = match
		}
	}

	return latest, nil
}

func loadDeploymentFromFile(filename string) (*appsv1.Deployment, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	deployment := &appsv1.Deployment{}
	if err := sigsyaml.Unmarshal(data, deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return deployment, nil
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
