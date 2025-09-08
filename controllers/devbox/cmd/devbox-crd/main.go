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
	"strings"
	"time"

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

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("devbox-crd")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

type CRDConfig struct {
	DryRun             bool
	CRDDir             string
	Action             string // "apply", "disable-v1alpha1", "check-status"
	OnlyDevboxes       bool
	OnlyReleases       bool
	WaitForReady       bool
	DisableOldVersions bool
}

func main() {
	var config CRDConfig
	flag.BoolVar(&config.DryRun, "dry-run", false, "If true, only print what would be done")
	flag.StringVar(&config.CRDDir, "crd-dir", "./scripts/manifests", "Directory containing CRD files")
	flag.StringVar(&config.Action, "action", "apply", "Action to perform: apply, disable-v1alpha1, check-status")
	flag.BoolVar(&config.OnlyDevboxes, "only-devboxes", false, "Only process devboxes CRD")
	flag.BoolVar(&config.OnlyReleases, "only-releases", false, "Only process devboxreleases CRD")
	flag.BoolVar(&config.WaitForReady, "wait", true, "Wait for CRDs to be ready")
	flag.BoolVar(&config.DisableOldVersions, "disable-old-versions", false, "Disable old versions after applying")

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

	setupLog.Info("Starting devbox CRD management",
		"dry-run", config.DryRun,
		"crd-dir", config.CRDDir,
		"action", config.Action,
		"only-devboxes", config.OnlyDevboxes,
		"only-releases", config.OnlyReleases)

	if err := performCRDOperation(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "CRD operation failed")
		os.Exit(1)
	}

	setupLog.Info("CRD operation completed successfully")
}

func performCRDOperation(ctx context.Context, k8sClient client.Client, config CRDConfig) error {
	switch config.Action {
	case "apply":
		return applyCRDs(ctx, k8sClient, config)
	case "disable-v1alpha1":
		return disableV1Alpha1Versions(ctx, k8sClient, config)
	case "check-status":
		return checkCRDStatus(ctx, k8sClient, config)
	default:
		return fmt.Errorf("unknown action: %s", config.Action)
	}
}

func applyCRDs(ctx context.Context, k8sClient client.Client, config CRDConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would apply CRDs from", "crd-dir", config.CRDDir)
		return nil
	}

	if !config.OnlyReleases {
		// 应用devboxes CRD
		devboxCRDFile := filepath.Join(config.CRDDir, "devbox_v1alpha2_crd.yaml")
		if err := applyCRDFromFile(ctx, k8sClient, devboxCRDFile, "devboxes"); err != nil {
			return fmt.Errorf("failed to apply devboxes CRD: %w", err)
		}
	}

	if !config.OnlyDevboxes {
		// 应用devboxreleases CRD
		devboxReleaseCRDFile := filepath.Join(config.CRDDir, "devboxrelease_v1alpha2_crd.yaml")
		if err := applyCRDFromFile(ctx, k8sClient, devboxReleaseCRDFile, "devboxreleases"); err != nil {
			return fmt.Errorf("failed to apply devboxreleases CRD: %w", err)
		}
	}

	if config.WaitForReady {
		setupLog.Info("Waiting for CRDs to be ready...")
		if err := waitForCRDsReady(ctx, k8sClient, config); err != nil {
			return fmt.Errorf("failed to wait for CRDs to be ready: %w", err)
		}
	}

	if config.DisableOldVersions {
		setupLog.Info("Disabling old versions...")
		disableConfig := config
		disableConfig.Action = "disable-v1alpha1"
		return disableV1Alpha1Versions(ctx, k8sClient, disableConfig)
	}

	setupLog.Info("Successfully applied CRDs")
	return nil
}

func applyCRDFromFile(ctx context.Context, k8sClient client.Client, filename, crdType string) error {
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
			setupLog.Info("Created CRD", "name", crd.Name, "type", crdType)
		} else {
			return fmt.Errorf("failed to get existing CRD %s: %w", crd.Name, err)
		}
	} else {
		// 更新现有CRD
		crd.ResourceVersion = existing.ResourceVersion
		if err := k8sClient.Update(ctx, crd); err != nil {
			return fmt.Errorf("failed to update CRD %s: %w", crd.Name, err)
		}
		setupLog.Info("Updated CRD", "name", crd.Name, "type", crdType)
	}

	return nil
}

func disableV1Alpha1Versions(ctx context.Context, k8sClient client.Client, config CRDConfig) error {
	if config.DryRun {
		setupLog.Info("DRY-RUN: Would disable v1alpha1 versions in CRDs")
		return nil
	}

	if !config.OnlyReleases {
		// 禁用devboxes CRD的v1alpha1版本
		if err := disableV1Alpha1Version(ctx, k8sClient, "devboxes.devbox.sealos.io"); err != nil {
			return fmt.Errorf("failed to disable v1alpha1 for devboxes CRD: %w", err)
		}
	}

	if !config.OnlyDevboxes {
		// 禁用devboxreleases CRD的v1alpha1版本
		if err := disableV1Alpha1Version(ctx, k8sClient, "devboxreleases.devbox.sealos.io"); err != nil {
			return fmt.Errorf("failed to disable v1alpha1 for devboxreleases CRD: %w", err)
		}
	}

	setupLog.Info("Successfully disabled v1alpha1 versions in CRDs")
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
			if crd.Spec.Versions[i].Served {
				crd.Spec.Versions[i].Served = false
				modified = true
				setupLog.Info("Disabled v1alpha1 serving", "crd", crdName)
			}
			break
		}
	}

	if !modified {
		setupLog.Info("v1alpha1 version already disabled or not found", "crd", crdName)
		return nil
	}

	if err := k8sClient.Update(ctx, crd); err != nil {
		return fmt.Errorf("failed to update CRD %s: %w", crdName, err)
	}

	setupLog.Info("Successfully disabled v1alpha1 version", "crd", crdName)
	return nil
}

func checkCRDStatus(ctx context.Context, k8sClient client.Client, config CRDConfig) error {
	crdNames := []string{}

	if !config.OnlyReleases {
		crdNames = append(crdNames, "devboxes.devbox.sealos.io")
	}
	if !config.OnlyDevboxes {
		crdNames = append(crdNames, "devboxreleases.devbox.sealos.io")
	}

	for _, crdName := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
			if errors.IsNotFound(err) {
				setupLog.Info("CRD not found", "name", crdName)
				continue
			}
			return fmt.Errorf("failed to get CRD %s: %w", crdName, err)
		}

		setupLog.Info("CRD Status", "name", crdName)
		setupLog.Info("  Storage Version", "version", crd.Status.StoredVersions)

		for _, version := range crd.Spec.Versions {
			setupLog.Info("  Version",
				"name", version.Name,
				"served", version.Served,
				"storage", version.Storage)
		}

		// 检查条件
		for _, condition := range crd.Status.Conditions {
			setupLog.Info("  Condition",
				"type", condition.Type,
				"status", condition.Status,
				"reason", condition.Reason)
		}

		fmt.Println() // 空行分隔
	}

	return nil
}

func waitForCRDsReady(ctx context.Context, k8sClient client.Client, config CRDConfig) error {
	crdNames := []string{}

	if !config.OnlyReleases {
		crdNames = append(crdNames, "devboxes.devbox.sealos.io")
	}
	if !config.OnlyDevboxes {
		crdNames = append(crdNames, "devboxreleases.devbox.sealos.io")
	}

	for _, crdName := range crdNames {
		setupLog.Info("Waiting for CRD to be established", "name", crdName)

		for i := 0; i < 60; i++ { // 最多等待60秒
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
				if errors.IsNotFound(err) {
					setupLog.Info("CRD not found, retrying...", "name", crdName)
					continue
				}
				return fmt.Errorf("failed to get CRD %s: %w", crdName, err)
			}

			// 检查是否已建立
			established := false
			for _, condition := range crd.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					established = true
					break
				}
			}

			if established {
				setupLog.Info("CRD is established", "name", crdName)
				break
			}

			if i == 59 {
				return fmt.Errorf("timeout waiting for CRD %s to be established", crdName)
			}

			time.Sleep(1 * time.Second)
		}
	}

	return nil
}
