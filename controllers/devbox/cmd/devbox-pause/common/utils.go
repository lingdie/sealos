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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
)

// SaveToFile 将对象保存为YAML文件
func SaveToFile(obj client.Object, filename string) error {
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

// SaveBackupStates 保存备份状态到JSON文件
func SaveBackupStates(states []DevboxBackupState, filename string) error {
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup states: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup states file: %w", filename, err)
	}

	return nil
}

// WaitForCommitsToFinish 等待所有commit操作完成
func WaitForCommitsToFinish(ctx context.Context, k8sClient client.Client, devbox *devboxv1alpha1.Devbox, timeout time.Duration, logger Logger) error {
	logger.Info("Waiting for commits to finish", "devbox", devbox.Name, "namespace", devbox.Namespace)

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
			logger.Info("All commits finished", "devbox", devbox.Name, "namespace", devbox.Namespace)
			return nil
		}

		logger.Info("Still waiting for commits to finish", "devbox", devbox.Name, "namespace", devbox.Namespace)
		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for commits to finish for devbox %s/%s", devbox.Namespace, devbox.Name)
}

// Logger 接口，用于统一日志输出
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}
