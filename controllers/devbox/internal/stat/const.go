package stat

import (
	"time"
)

const (
	ThinPoolName = "devbox-vg-thinpool"
	LVMBackupDir = "/etc/lvm/backup"
	VGName       = "devbox-vg"

	DefaultThinPoolMonitoringInterval = 10 * time.Second
	DefaultLVMClientAddr              = "localhost:9090"
	DefaultLVMClientTimeout           = 10 * time.Second
	DefaultLVMClientRetryCount        = 3
	DefaultLVMClientRetryInterval     = 5 * time.Second

	DefaultGRPCPort        = 9090
	DefaultMonitorInterval = 5 * time.Second

	VMImportURL = "http://vmsingle-victoria-metrics-k8s-stack.vm.svc.cluster.local:8429/api/v1/import/prometheus"
)
