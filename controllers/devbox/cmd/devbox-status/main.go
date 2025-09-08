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
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/runtime"
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
	setupLog = ctrl.Log.WithName("devbox-status")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha1.AddToScheme(scheme))
	utilruntime.Must(devboxv1alpha2.AddToScheme(scheme))
}

type StatusConfig struct {
	Namespace     string
	OutputFormat  string // "table", "json", "yaml"
	OnlyUpgrading bool
	ShowAll       bool
	DevboxName    string
}

func main() {
	var config StatusConfig
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to check (empty for all namespaces)")
	flag.StringVar(&config.OutputFormat, "output", "table", "Output format: table, json, yaml")
	flag.BoolVar(&config.OnlyUpgrading, "only-upgrading", false, "Only show devboxes that are upgrading")
	flag.BoolVar(&config.ShowAll, "all", false, "Show all devboxes including those without upgrade annotations")
	flag.StringVar(&config.DevboxName, "devbox", "", "Specific devbox name to check")

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

	setupLog.Info("Checking devbox upgrade status",
		"namespace", config.Namespace,
		"output-format", config.OutputFormat,
		"only-upgrading", config.OnlyUpgrading,
		"show-all", config.ShowAll,
		"devbox", config.DevboxName)

	if err := checkUpgradeStatus(ctx, k8sClient, config); err != nil {
		setupLog.Error(err, "failed to check upgrade status")
		os.Exit(1)
	}
}

func checkUpgradeStatus(ctx context.Context, k8sClient client.Client, config StatusConfig) error {
	devboxList := &devboxv1alpha1.DevboxList{}
	listOpts := []client.ListOption{}

	if config.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(config.Namespace))
	}

	if err := k8sClient.List(ctx, devboxList, listOpts...); err != nil {
		return fmt.Errorf("failed to list devboxes: %w", err)
	}

	// 过滤结果
	var filteredDevboxes []devboxv1alpha1.Devbox
	for _, devbox := range devboxList.Items {
		// 如果指定了特定的devbox名称
		if config.DevboxName != "" && devbox.Name != config.DevboxName {
			continue
		}

		upgradeInfo := upgrade.GetUpgradeInfo(&devbox)

		// 如果只显示正在升级的devbox
		if config.OnlyUpgrading && upgradeInfo.Status == "" {
			continue
		}

		// 如果不显示所有devbox，只显示有升级annotation的
		if !config.ShowAll && upgradeInfo.Status == "" {
			continue
		}

		filteredDevboxes = append(filteredDevboxes, devbox)
	}

	switch config.OutputFormat {
	case "table":
		return displayTable(filteredDevboxes)
	case "json":
		return displayJSON(filteredDevboxes)
	case "yaml":
		return displayYAML(filteredDevboxes)
	default:
		return fmt.Errorf("unsupported output format: %s", config.OutputFormat)
	}
}

func displayTable(devboxes []devboxv1alpha1.Devbox) error {
	if len(devboxes) == 0 {
		fmt.Println("No devboxes found matching criteria.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tNAME\tCURRENT STATE\tORIGINAL STATE\tUPGRADE STATUS\tUPGRADE STEP\tOPERATION ID\tPROGRESS\tTIMESTAMP")
	fmt.Fprintln(w, strings.Repeat("-", 120))

	for _, devbox := range devboxes {
		upgradeInfo := upgrade.GetUpgradeInfo(&devbox)

		currentState := string(devbox.Spec.State)
		originalState := upgradeInfo.OriginalState
		if originalState == "" {
			originalState = "-"
		}

		upgradeStatus := upgradeInfo.Status
		if upgradeStatus == "" {
			upgradeStatus = "-"
		}

		upgradeStep := upgradeInfo.Step
		if upgradeStep == "" {
			upgradeStep = "-"
		}

		operationID := upgradeInfo.OperationID
		if operationID == "" {
			operationID = "-"
		}

		progress := upgradeInfo.Progress
		if progress == "" {
			progress = "-"
		}

		timestamp := devbox.Annotations[upgrade.AnnotationUpgradeTimestamp]
		if timestamp == "" {
			timestamp = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			devbox.Namespace,
			devbox.Name,
			currentState,
			originalState,
			upgradeStatus,
			upgradeStep,
			operationID,
			progress,
			timestamp)
	}

	return w.Flush()
}

func displayJSON(devboxes []devboxv1alpha1.Devbox) error {
	type DevboxStatus struct {
		Namespace     string              `json:"namespace"`
		Name          string              `json:"name"`
		CurrentState  string              `json:"currentState"`
		OriginalState string              `json:"originalState,omitempty"`
		UpgradeInfo   upgrade.UpgradeInfo `json:"upgradeInfo"`
		Timestamp     string              `json:"timestamp,omitempty"`
	}

	var statuses []DevboxStatus
	for _, devbox := range devboxes {
		upgradeInfo := upgrade.GetUpgradeInfo(&devbox)
		timestamp := devbox.Annotations[upgrade.AnnotationUpgradeTimestamp]

		status := DevboxStatus{
			Namespace:     devbox.Namespace,
			Name:          devbox.Name,
			CurrentState:  string(devbox.Spec.State),
			OriginalState: upgradeInfo.OriginalState,
			UpgradeInfo:   upgradeInfo,
			Timestamp:     timestamp,
		}
		statuses = append(statuses, status)
	}

	// 简单的JSON输出
	fmt.Println("{")
	fmt.Printf("  \"devboxes\": [\n")
	for i, status := range statuses {
		fmt.Printf("    {\n")
		fmt.Printf("      \"namespace\": \"%s\",\n", status.Namespace)
		fmt.Printf("      \"name\": \"%s\",\n", status.Name)
		fmt.Printf("      \"currentState\": \"%s\",\n", status.CurrentState)
		if status.OriginalState != "" {
			fmt.Printf("      \"originalState\": \"%s\",\n", status.OriginalState)
		}
		fmt.Printf("      \"upgradeStatus\": \"%s\",\n", status.UpgradeInfo.Status)
		fmt.Printf("      \"upgradeStep\": \"%s\",\n", status.UpgradeInfo.Step)
		fmt.Printf("      \"operationId\": \"%s\",\n", status.UpgradeInfo.OperationID)
		fmt.Printf("      \"progress\": \"%s\",\n", status.UpgradeInfo.Progress)
		fmt.Printf("      \"timestamp\": \"%s\"\n", status.Timestamp)
		if i < len(statuses)-1 {
			fmt.Printf("    },\n")
		} else {
			fmt.Printf("    }\n")
		}
	}
	fmt.Printf("  ]\n")
	fmt.Println("}")

	return nil
}

func displayYAML(devboxes []devboxv1alpha1.Devbox) error {
	fmt.Println("devboxes:")
	for _, devbox := range devboxes {
		upgradeInfo := upgrade.GetUpgradeInfo(&devbox)
		timestamp := devbox.Annotations[upgrade.AnnotationUpgradeTimestamp]

		fmt.Printf("- namespace: %s\n", devbox.Namespace)
		fmt.Printf("  name: %s\n", devbox.Name)
		fmt.Printf("  currentState: %s\n", devbox.Spec.State)
		if upgradeInfo.OriginalState != "" {
			fmt.Printf("  originalState: %s\n", upgradeInfo.OriginalState)
		}
		fmt.Printf("  upgradeStatus: %s\n", upgradeInfo.Status)
		fmt.Printf("  upgradeStep: %s\n", upgradeInfo.Step)
		fmt.Printf("  operationId: %s\n", upgradeInfo.OperationID)
		fmt.Printf("  progress: %s\n", upgradeInfo.Progress)
		fmt.Printf("  timestamp: %s\n", timestamp)
		fmt.Println()
	}

	return nil
}
