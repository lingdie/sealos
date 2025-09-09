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
	"time"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
)

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

type BaseConfig struct {
	DryRun    bool
	BackupDir string
}
