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
	Name        string                     `json:"name"`
	Namespace   string                     `json:"namespace"`
	State       devboxv1alpha1.DevboxState `json:"state"`
	Phase       devboxv1alpha1.DevboxPhase `json:"phase"`
	OperationID string                     `json:"operationId"`
	BackupTime  time.Time                  `json:"backupTime"`
}

// BaseConfig 基础配置结构，包含所有工具的通用配置
type BaseConfig struct {
	DryRun    bool
	Namespace string
}

// BackupConfig 备份工具配置
type BackupConfig struct {
	BaseConfig
	BackupDir string
}

// RestoreConfig 恢复工具配置
type RestoreConfig struct {
	BaseConfig
	BackupDir        string
	BackupStatesFile string
	OperationID      string
	OnlyStates       bool
	Force            bool
}

// StatusConfig 状态查看工具配置
type StatusConfig struct {
	BaseConfig
	OutputFormat  string // "table", "json", "yaml"
	OnlyUpgrading bool
	ShowAll       bool
	DevboxName    string
}

// TransformConfig 转换工具配置
type TransformConfig struct {
	BaseConfig
	OnlyDevboxes      bool
	OnlyReleases      bool
	BatchSize         int
	DelayBetweenBatch time.Duration
}

// CRDConfig CRD管理工具配置
type CRDConfig struct {
	BaseConfig
	CRDDir             string
	Action             string // "apply", "disable-v1alpha1", "check-status"
	OnlyDevboxes       bool
	OnlyReleases       bool
	WaitForReady       bool
	DisableOldVersions bool
}

// Logger 接口，用于统一日志输出
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}
