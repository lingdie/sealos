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
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
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
	setupLog = ctrl.Log.WithName("upgrade")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
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

type UpgradeConfig struct {
	DryRun        bool
	Namespace     string
	BackupDir     string
	CRDDir        string
	SkipBackup    bool
	SkipPause     bool
	OnlyBackup    bool
	OnlyPause     bool
	OnlyTransform bool
}

func main() {
	var config UpgradeConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to upgrade (empty for all namespaces)")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")
	flag.StringVar(&config.CRDDir, "crd-dir", "./scripts/manifests", "Directory containing CRD files")
	flag.BoolVar(&config.SkipBackup, "skip-backup", false, "Skip backup step")
	flag.BoolVar(&config.SkipPause, "skip-pause", false, "Skip pause step")
	flag.BoolVar(&config.OnlyBackup, "only-backup", false, "Only perform backup")
	flag.BoolVar(&config.OnlyPause, "only-pause", false, "Only pause devboxes")
	flag.BoolVar(&config.OnlyTransform, "only-transform", false, "Only transform CR")

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

	setupLog.Info("Starting devbox v1alpha1 to v1alpha2 upgrade process",
		"dry-run", config.DryRun,
		"namespace", config.Namespace,
		"backup-dir", config.BackupDir,
		"crd-dir", config.CRDDir)

	// 执行升级流程
	if err := performUpgrade(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "upgrade process failed")
		os.Exit(1)
	}

	setupLog.Info("Upgrade process completed successfully")
}

func performUpgrade(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	// 1. 备份CR & CRD
	if !config.SkipBackup && !config.OnlyPause && !config.OnlyTransform {
		setupLog.Info("Step 1: Backing up CR & CRD")
		if err := backupCRAndCRD(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to backup CR & CRD: %w", err)
		}
		if config.OnlyBackup {
			return nil
		}
	}

	// 2. 暂停所有devbox，等待commit结束（记录操作id及原状态）
	if !config.SkipPause && !config.OnlyBackup && !config.OnlyTransform {
		setupLog.Info("Step 2: Pausing all devboxes and waiting for commits to finish")
		if err := pauseAllDevboxes(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to pause devboxes: %w", err)
		}
		if config.OnlyPause {
			return nil
		}
	}

	// 3. 暂停Controller（直接删除旧的devbox deployment）
	if !config.OnlyBackup && !config.OnlyPause && !config.OnlyTransform {
		setupLog.Info("Step 3: Pausing controller")
		if err := pauseController(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to pause controller: %w", err)
		}
	}

	// 4. CRD更新（使用scripts/manifests下的crd文件）
	if !config.OnlyBackup && !config.OnlyPause && !config.OnlyTransform {
		setupLog.Info("Step 4: Updating CRDs")
		if err := updateCRDs(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to update CRDs: %w", err)
		}
	}

	// 5. CR transform（使用cmd/upgrade）
	if !config.OnlyBackup && !config.OnlyPause {
		setupLog.Info("Step 5: Transforming CRs")
		if err := transformCRs(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to transform CRs: %w", err)
		}
		if config.OnlyTransform {
			return nil
		}
	}

	// 6. CRD更新（设置v1alpha1版本served=false，并从storedVersions中删除）
	if !config.OnlyBackup && !config.OnlyPause && !config.OnlyTransform {
		setupLog.Info("Step 6: Final CRD update - disable v1alpha1")
		if err := finalCRDUpdate(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to perform final CRD update: %w", err)
		}
	}

	return nil
}

// backupCRAndCRD 备份所有CR和CRD
func backupCRAndCRD(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would backup CR & CRD to", "backup-dir", config.BackupDir)
		return nil
	}

	// 创建备份目录
	if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 备份CRD
	if err := backupCRDs(ctx, k8sClient, config.BackupDir); err != nil {
		return fmt.Errorf("failed to backup CRDs: %w", err)
	}

	// 备份Devboxes
	if err := backupDevboxes(ctx, k8sClient, config); err != nil {
		return fmt.Errorf("failed to backup Devboxes: %w", err)
	}

	// 备份DevboxReleases
	if err := backupDevboxReleases(ctx, k8sClient, config); err != nil {
		return fmt.Errorf("failed to backup DevboxReleases: %w", err)
	}

	setupLog.Info("Successfully backed up CR & CRD", "backup-dir", config.BackupDir)
	return nil
}

func backupCRDs(ctx context.Context, k8sClient client.Client, backupDir string) error {
	// 备份devboxes CRD
	devboxCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "devboxes.devbox.sealos.io"}, devboxCRD); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get devboxes CRD: %w", err)
		}
	} else {
		if err := saveToFile(devboxCRD, filepath.Join(backupDir, "devboxes_crd.yaml")); err != nil {
			return fmt.Errorf("failed to save devboxes CRD: %w", err)
		}
	}

	// 备份devboxreleases CRD
	devboxReleaseCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "devboxreleases.devbox.sealos.io"}, devboxReleaseCRD); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get devboxreleases CRD: %w", err)
		}
	} else {
		if err := saveToFile(devboxReleaseCRD, filepath.Join(backupDir, "devboxreleases_crd.yaml")); err != nil {
			return fmt.Errorf("failed to save devboxreleases CRD: %w", err)
		}
	}

	return nil
}

func backupDevboxes(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Devboxes: %w", err)
	}

	setupLog.Info("Backing up Devboxes", "count", len(devboxList.Items))

	for i, devbox := range devboxList.Items {
		filename := fmt.Sprintf("devbox_%s_%s.yaml", devbox.Namespace, devbox.Name)
		if err := saveToFile(&devbox, filepath.Join(config.BackupDir, filename)); err != nil {
			return fmt.Errorf("failed to backup devbox %s/%s: %w", devbox.Namespace, devbox.Name, err)
		}

		if i%10 == 0 {
			setupLog.Info("Backup progress", "processed", i+1, "total", len(devboxList.Items))
		}
	}

	return nil
}

func backupDevboxReleases(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	devboxReleaseList := &devboxv1alpha1.DevBoxReleaseList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxReleaseList, listOpts...); err != nil {
		return fmt.Errorf("failed to list DevboxReleases: %w", err)
	}

	setupLog.Info("Backing up DevboxReleases", "count", len(devboxReleaseList.Items))

	for i, devboxRelease := range devboxReleaseList.Items {
		filename := fmt.Sprintf("devboxrelease_%s_%s.yaml", devboxRelease.Namespace, devboxRelease.Name)
		if err := saveToFile(&devboxRelease, filepath.Join(config.BackupDir, filename)); err != nil {
			return fmt.Errorf("failed to backup devboxrelease %s/%s: %w", devboxRelease.Namespace, devboxRelease.Name, err)
		}

		if i%10 == 0 {
			setupLog.Info("Backup progress", "processed", i+1, "total", len(devboxReleaseList.Items))
		}
	}

	return nil
}

// pauseAllDevboxes 暂停所有devbox，等待commit结束（记录操作id及原状态）
func pauseAllDevboxes(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
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
	operationID := fmt.Sprintf("upgrade-%d", time.Now().Unix())

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

		// 如果devbox正在运行，需要暂停它
		if devbox.Spec.State == devboxv1alpha1.DevboxStateRunning {
			// 等待commit结束
			if err := waitForCommitsToFinish(ctx, k8sClient, devbox); err != nil {
				setupLog.Error(err, "Failed to wait for commits to finish",
					"name", devbox.Name,
					"namespace", devbox.Namespace)
				return err
			}

			// 设置为Stopped状态
			devbox.Spec.State = devboxv1alpha1.DevboxStateStopped
			if err := k8sClient.Update(ctx, devbox); err != nil {
				return fmt.Errorf("failed to pause devbox %s/%s: %w", devbox.Namespace, devbox.Name, err)
			}

			setupLog.Info("Successfully paused Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace,
				"operation-id", operationID)
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
func waitForCommitsToFinish(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox) error {
	setupLog.Info("Waiting for commits to finish", "devbox", devbox.Name, "namespace", devbox.Namespace)

	timeout := 5 * time.Minute
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
func pauseController(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	deploymentName := "devbox-controller-manager"
	namespace := "devbox-system"

	if config.DryRun {
		setupLog.Info("DRY-RUN: Would delete controller deployment",
			"deployment", deploymentName,
			"namespace", namespace)
		return nil
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			setupLog.Info("Controller deployment not found, might be already deleted",
				"deployment", deploymentName,
				"namespace", namespace)
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
		"deployment", deploymentName,
		"namespace", namespace,
		"backup-file", backupFile)

	return nil
}

// updateCRDs 更新CRD（使用scripts/manifests下的crd文件）
func updateCRDs(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would update CRDs from", "crd-dir", config.CRDDir)
		return nil
	}

	// 更新devboxes CRD
	devboxCRDFile := filepath.Join(config.CRDDir, "devbox_v1alpha2_crd.yaml")
	if err := applyCRDFromFile(ctx, k8sClient, devboxCRDFile); err != nil {
		return fmt.Errorf("failed to update devboxes CRD: %w", err)
	}

	// 更新devboxreleases CRD
	devboxReleaseCRDFile := filepath.Join(config.CRDDir, "devboxrelease_v1alpha2_crd.yaml")
	if err := applyCRDFromFile(ctx, k8sClient, devboxReleaseCRDFile); err != nil {
		return fmt.Errorf("failed to update devboxreleases CRD: %w", err)
	}

	setupLog.Info("Successfully updated CRDs")
	return nil
}

func applyCRDFromFile(ctx context.Context, k8sClient client.Client, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read CRD file %s: %w", filename, err)
	}

	// 解析YAML
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := decoder.Decode(crd); err != nil {
		return fmt.Errorf("failed to decode CRD from %s: %w", filename, err)
	}

	// 检查CRD是否存在
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: crd.Name}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			// 创建新的CRD
			if err := k8sClient.Create(ctx, crd); err != nil {
				return fmt.Errorf("failed to create CRD %s: %w", crd.Name, err)
			}
			setupLog.Info("Created CRD", "name", crd.Name)
		} else {
			return fmt.Errorf("failed to get existing CRD %s: %w", crd.Name, err)
		}
	} else {
		// 更新现有CRD
		crd.ResourceVersion = existing.ResourceVersion
		if err := k8sClient.Update(ctx, crd); err != nil {
			return fmt.Errorf("failed to update CRD %s: %w", crd.Name, err)
		}
		setupLog.Info("Updated CRD", "name", crd.Name)
	}

	return nil
}

// transformCRs CR transform（使用cmd/upgrade）
func transformCRs(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	setupLog.Info("Transforming CRs from v1alpha1 to v1alpha2")

	if err := transformDevboxes(ctx, k8sClient, config); err != nil {
		return fmt.Errorf("failed to transform Devboxes: %w", err)
	}

	if err := transformDevboxReleases(ctx, k8sClient, config); err != nil {
		return fmt.Errorf("failed to transform DevboxReleases: %w", err)
	}

	setupLog.Info("Successfully transformed all CRs")
	return nil
}

func transformDevboxes(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Devboxes: %w", err)
	}

	setupLog.Info("Transforming Devboxes", "count", len(devboxList.Items))

	for i := range devboxList.Items {
		devbox := &devboxList.Items[i]

		setupLog.Info("Transforming Devbox",
			"name", devbox.Name,
			"namespace", devbox.Namespace)

		if config.DryRun {
			setupLog.Info("DRY-RUN: Would transform Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace)
			continue
		}

		// 强制存储版本转换
		if err := forceStorageVersionUpdate(ctx, k8sClient, devbox); err != nil {
			setupLog.Error(err, "Failed to transform Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace)
			return err
		}

		setupLog.Info("Successfully transformed Devbox",
			"name", devbox.Name,
			"namespace", devbox.Namespace)

		// 添加延迟避免过载API server
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func transformDevboxReleases(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	devboxReleaseList := &devboxv1alpha1.DevBoxReleaseList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxReleaseList, listOpts...); err != nil {
		return fmt.Errorf("failed to list DevboxReleases: %w", err)
	}

	setupLog.Info("Transforming DevboxReleases", "count", len(devboxReleaseList.Items))

	for i := range devboxReleaseList.Items {
		devboxRelease := &devboxReleaseList.Items[i]

		setupLog.Info("Transforming DevboxRelease",
			"name", devboxRelease.Name,
			"namespace", devboxRelease.Namespace)

		if config.DryRun {
			setupLog.Info("DRY-RUN: Would transform DevboxRelease",
				"name", devboxRelease.Name,
				"namespace", devboxRelease.Namespace)
			continue
		}

		// 强制存储版本转换
		if err := forceStorageVersionUpdateRelease(ctx, k8sClient, devboxRelease); err != nil {
			setupLog.Error(err, "Failed to transform DevboxRelease",
				"name", devboxRelease.Name,
				"namespace", devboxRelease.Namespace)
			return err
		}

		setupLog.Info("Successfully transformed DevboxRelease",
			"name", devboxRelease.Name,
			"namespace", devboxRelease.Namespace)

		// 添加延迟避免过载API server
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// finalCRDUpdate CRD更新（设置v1alpha1版本served=false，并从storedVersions中删除）
func finalCRDUpdate(ctx context.Context, k8sClient client.Client, config UpgradeConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would disable v1alpha1 version in CRDs")
		return nil
	}

	// 更新devboxes CRD
	if err := disableV1Alpha1Version(ctx, k8sClient, "devboxes.devbox.sealos.io"); err != nil {
		return fmt.Errorf("failed to disable v1alpha1 for devboxes CRD: %w", err)
	}

	// 更新devboxreleases CRD
	if err := disableV1Alpha1Version(ctx, k8sClient, "devboxreleases.devbox.sealos.io"); err != nil {
		return fmt.Errorf("failed to disable v1alpha1 for devboxreleases CRD: %w", err)
	}

	setupLog.Info("Successfully disabled v1alpha1 version in CRDs")
	return nil
}

func disableV1Alpha1Version(ctx context.Context, k8sClient client.Client, crdName string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
		return fmt.Errorf("failed to get CRD %s: %w", crdName, err)
	}

	// 找到v1alpha1版本并设置served=false
	modified := false
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == "v1alpha1" {
			crd.Spec.Versions[i].Served = false
			modified = true
			break
		}
	}

	if !modified {
		setupLog.Info("v1alpha1 version not found in CRD", "crd", crdName)
		return nil
	}

	// 从storedVersions中删除v1alpha1
	var newStoredVersions []string
	for _, version := range crd.Status.StoredVersions {
		if version != "v1alpha1" {
			newStoredVersions = append(newStoredVersions, version)
		}
	}
	crd.Status.StoredVersions = newStoredVersions

	if err := k8sClient.Update(ctx, crd); err != nil {
		return fmt.Errorf("failed to update CRD %s: %w", crdName, err)
	}

	setupLog.Info("Disabled v1alpha1 version", "crd", crdName)
	return nil
}

func forceStorageVersionUpdate(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox) error {
	// Get the latest version of the object
	key := types.NamespacedName{
		Namespace: devbox.Namespace,
		Name:      devbox.Name,
	}

	latest := &devboxv1alpha1.Devbox{}
	if err := k8sClient.Get(ctx, key, latest); err != nil {
		return fmt.Errorf("failed to get latest Devbox: %w", err)
	}

	// Update the object to force storage version conversion
	// We'll add/update an annotation to trigger the write
	if latest.Annotations == nil {
		latest.Annotations = make(map[string]string)
	}
	latest.Annotations["devbox.sealos.io/storage-upgrade"] = fmt.Sprintf("upgrade-%s-%d", latest.Name, time.Now().Unix())

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to update Devbox: %w", err)
	}

	return nil
}

func forceStorageVersionUpdateRelease(ctx context.Context, k8sClient client.Client, devboxRelease *devboxv1alpha1.DevBoxRelease) error {
	// Get the latest version of the object
	key := types.NamespacedName{
		Namespace: devboxRelease.Namespace,
		Name:      devboxRelease.Name,
	}

	latest := &devboxv1alpha1.DevBoxRelease{}
	if err := k8sClient.Get(ctx, key, latest); err != nil {
		return fmt.Errorf("failed to get latest DevboxRelease: %w", err)
	}

	// Update the object to force storage version conversion
	// We'll add/update an annotation to trigger the write
	if latest.Annotations == nil {
		latest.Annotations = make(map[string]string)
	}
	latest.Annotations["devbox.sealos.io/storage-upgrade"] = fmt.Sprintf("upgrade-%s-%d", latest.Name, time.Now().Unix())

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to update DevboxRelease: %w", err)
	}

	return nil
}

// 工具函数

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
