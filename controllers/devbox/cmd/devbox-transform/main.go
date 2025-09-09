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
	setupLog = ctrl.Log.WithName("devbox-transform")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

type TransformConfig struct {
	DryRun            bool
	Namespace         string
	OnlyDevboxes      bool
	OnlyReleases      bool
	BatchSize         int
	DelayBetweenBatch time.Duration
}

func main() {
	var config TransformConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to transform (empty for all namespaces)")
	flag.BoolVar(&config.OnlyDevboxes, "only-devboxes", false, "Only transform devboxes")
	flag.BoolVar(&config.OnlyReleases, "only-releases", false, "Only transform devbox releases")
	flag.IntVar(&config.BatchSize, "batch-size", 10, "Number of resources to process in each batch")
	flag.DurationVar(&config.DelayBetweenBatch, "delay", 1*time.Second, "Delay between batches")

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

	setupLog.Info("Starting devbox CR transform process",
		"dry-run", config.DryRun,
		"namespace", config.Namespace,
		"only-devboxes", config.OnlyDevboxes,
		"only-releases", config.OnlyReleases,
		"batch-size", config.BatchSize)

	if err := performTransform(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "transform process failed")
		os.Exit(1)
	}

	setupLog.Info("Transform process completed successfully")
}

func performTransform(ctx context.Context, k8sClient client.Client, config TransformConfig) error {
	if !config.OnlyReleases {
		setupLog.Info("Transforming Devboxes from v1alpha1 to v1alpha2")
		if err := transformDevboxes(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to transform Devboxes: %w", err)
		}
	}

	if !config.OnlyDevboxes {
		setupLog.Info("Transforming DevboxReleases from v1alpha1 to v1alpha2")
		if err := transformDevboxReleases(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to transform DevboxReleases: %w", err)
		}
	}

	setupLog.Info("Successfully transformed all CRs")
	return nil
}

func transformDevboxes(ctx context.Context, k8sClient client.Client, config TransformConfig) error {
	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Devboxes: %w", err)
	}

	setupLog.Info("Found Devboxes to transform", "count", len(devboxList.Items))

	// 批量处理
	for i := 0; i < len(devboxList.Items); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(devboxList.Items) {
			end = len(devboxList.Items)
		}

		setupLog.Info("Processing batch", "batch", fmt.Sprintf("%d-%d", i+1, end), "total", len(devboxList.Items))

		for j := i; j < end; j++ {
			devbox := &devboxList.Items[j]

			setupLog.Info("Transforming Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace,
				"progress", fmt.Sprintf("%d/%d", j+1, len(devboxList.Items)))

			if config.DryRun {
				setupLog.Info("DRY-RUN: Would transform Devbox",
					"name", devbox.Name,
					"namespace", devbox.Namespace)
				continue
			}

			// 更新annotation为transform步骤
			operationID := fmt.Sprintf("transform-%d", time.Now().Unix())
			upgradeInfo := upgrade.UpgradeInfo{
				OperationID: operationID,
				Step:        upgrade.UpgradeStepTransform,
				Status:      upgrade.UpgradeStatusInProgress,
				Version:     "v1alpha1-to-v1alpha2",
				Progress:    fmt.Sprintf("%d/%d", j+1, len(devboxList.Items)),
			}
			if err := upgrade.AddUpgradeAnnotations(ctx, k8sClient, devbox, upgradeInfo); err != nil {
				setupLog.Error(err, "Failed to add transform annotations",
					"name", devbox.Name,
					"namespace", devbox.Namespace)
				// 继续处理，不中断整个流程
			}

			// 强制存储版本转换
			if err := forceStorageVersionUpdate(ctx, k8sClient, devbox); err != nil {
				setupLog.Error(err, "Failed to transform Devbox",
					"name", devbox.Name,
					"namespace", devbox.Namespace)
				// 标记为失败
				if updateErr := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusFailed); updateErr != nil {
					setupLog.Error(updateErr, "Failed to update upgrade status annotation")
				}
				return err
			}

			// 标记转换完成
			if err := upgrade.UpdateUpgradeAnnotation(ctx, k8sClient, devbox, upgrade.AnnotationUpgradeStatus, upgrade.UpgradeStatusCompleted); err != nil {
				setupLog.Error(err, "Failed to update upgrade status annotation")
			}

			setupLog.Info("Successfully transformed Devbox",
				"name", devbox.Name,
				"namespace", devbox.Namespace)

			// 添加小延迟避免过载API server
			time.Sleep(100 * time.Millisecond)
		}

		// 批次间延迟
		if i+config.BatchSize < len(devboxList.Items) {
			setupLog.Info("Waiting between batches", "delay", config.DelayBetweenBatch)
			time.Sleep(config.DelayBetweenBatch)
		}
	}

	return nil
}

func transformDevboxReleases(ctx context.Context, k8sClient client.Client, config TransformConfig) error {
	devboxReleaseList := &devboxv1alpha1.DevBoxReleaseList{}
	listOpts := []client.ListOption{}
	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxReleaseList, listOpts...); err != nil {
		return fmt.Errorf("failed to list DevboxReleases: %w", err)
	}

	setupLog.Info("Found DevboxReleases to transform", "count", len(devboxReleaseList.Items))

	// 批量处理
	for i := 0; i < len(devboxReleaseList.Items); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(devboxReleaseList.Items) {
			end = len(devboxReleaseList.Items)
		}

		setupLog.Info("Processing batch", "batch", fmt.Sprintf("%d-%d", i+1, end), "total", len(devboxReleaseList.Items))

		for j := i; j < end; j++ {
			devboxRelease := &devboxReleaseList.Items[j]

			setupLog.Info("Transforming DevboxRelease",
				"name", devboxRelease.Name,
				"namespace", devboxRelease.Namespace,
				"progress", fmt.Sprintf("%d/%d", j+1, len(devboxReleaseList.Items)))

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

			// 添加小延迟避免过载API server
			time.Sleep(100 * time.Millisecond)
		}

		// 批次间延迟
		if i+config.BatchSize < len(devboxReleaseList.Items) {
			setupLog.Info("Waiting between batches", "delay", config.DelayBetweenBatch)
			time.Sleep(config.DelayBetweenBatch)
		}
	}

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
	latest.Annotations["devbox.sealos.io/storage-upgrade"] = fmt.Sprintf("transform-%s-%d", latest.Name, time.Now().Unix())

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
	latest.Annotations["devbox.sealos.io/storage-upgrade"] = fmt.Sprintf("transform-%s-%d", latest.Name, time.Now().Unix())

	if err := k8sClient.Update(ctx, latest); err != nil {
		return fmt.Errorf("failed to update DevboxRelease: %w", err)
	}

	return nil
}
