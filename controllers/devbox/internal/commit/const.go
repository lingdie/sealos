package commit

import "time"

const (
	DefaultNamespace           = "sealos.io"
	DefaultContainerdAddress   = "unix:///var/run/containerd/containerd.sock"
	DefaultDataRoot            = "/var/lib/containerd"
	DefaultRuntime             = "io.containerd.runc.v2"
	DefaultSnapshotter         = "devbox"
	DefaultNetworkMode         = "none"

  DefaultMaxRetries = 3
	DefaultRetryDelay = 5 * time.Second
	DefaultGcInterval = 20 * time.Minute

	InsecureRegistry           = true
	PauseContainerDuringCommit = false

	AnnotationKeyNamespace               = "namespace"
	AnnotationKeyImageName               = "image.name"
	AnnotationImageFromValue             = "true"
	AnnotationUseLimitValue              = "1Gi"
  DevboxOptionsRemoveBaseImageTopLayer = true

	SnapshotLabelPrefix  = "containerd.io/snapshot/devbox-"
	ContainerLabelPrefix = "devbox.sealos.io/"
)
