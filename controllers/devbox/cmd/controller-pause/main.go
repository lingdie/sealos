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

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
	"github.com/labring/sealos/controllers/devbox/cmd/devbox-pause/common"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("controller-pause")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

type ControllerPauseConfig struct {
	common.BaseConfig
	ControllerNS   string
	ControllerName string
}

func main() {
	var config ControllerPauseConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.BackupDir, "backup-dir", "./backup", "Directory to store backup files")
	flag.StringVar(&config.ControllerNS, "controller-namespace", "devbox-system", "Controller namespace")
	flag.StringVar(&config.ControllerName, "controller-name", "devbox-controller-manager", "Controller deployment name")

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

	setupLog.Info("Starting controller pause process",
		"dry-run", config.DryRun,
		"backup-dir", config.BackupDir,
		"controller-namespace", config.ControllerNS,
		"controller-name", config.ControllerName)

	if err := pauseController(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "controller pause process failed")
		os.Exit(1)
	}

	setupLog.Info("Controller pause process completed successfully")
}

// pauseController 暂停Controller（直接删除旧的devbox deployment）
func pauseController(ctx context.Context, k8sClient client.Client, config ControllerPauseConfig) error {
	// 创建备份目录
	if !config.DryRun {
		if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
	}

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
	if err := common.SaveToFile(deployment, backupFile); err != nil {
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
