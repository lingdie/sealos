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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	setupLog = ctrl.Log.WithName("devbox-backup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

type BackupConfig struct {
	DryRun    bool
	Namespace string
	BackupDir string
}

func main() {
	var config BackupConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to backup (empty for all namespaces)")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")

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

	setupLog.Info("Starting devbox backup process",
		"dry-run", config.DryRun,
		"namespace", config.Namespace,
		"backup-dir", config.BackupDir)

	if err := performBackup(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "backup process failed")
		os.Exit(1)
	}

	setupLog.Info("Backup process completed successfully")
}

func performBackup(ctx context.Context, k8sClient client.Client, config BackupConfig) error {
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
		setupLog.Info("devboxes CRD not found, skipping backup")
	} else {
		if err := saveToFile(devboxCRD, filepath.Join(backupDir, "devboxes_crd.yaml")); err != nil {
			return fmt.Errorf("failed to save devboxes CRD: %w", err)
		}
		setupLog.Info("Backed up devboxes CRD")
	}

	// 备份devboxreleases CRD
	devboxReleaseCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "devboxreleases.devbox.sealos.io"}, devboxReleaseCRD); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get devboxreleases CRD: %w", err)
		}
		setupLog.Info("devboxreleases CRD not found, skipping backup")
	} else {
		if err := saveToFile(devboxReleaseCRD, filepath.Join(backupDir, "devboxreleases_crd.yaml")); err != nil {
			return fmt.Errorf("failed to save devboxreleases CRD: %w", err)
		}
		setupLog.Info("Backed up devboxreleases CRD")
	}

	return nil
}

func backupDevboxes(ctx context.Context, k8sClient client.Client, config BackupConfig) error {
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

		if i%10 == 0 && i > 0 {
			setupLog.Info("Backup progress", "processed", i+1, "total", len(devboxList.Items))
		}
	}

	return nil
}

func backupDevboxReleases(ctx context.Context, k8sClient client.Client, config BackupConfig) error {
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

		if i%10 == 0 && i > 0 {
			setupLog.Info("Backup progress", "processed", i+1, "total", len(devboxReleaseList.Items))
		}
	}

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
