package commit

const (
	DefaultNamespace           = "sealos.io"
	DefaultContainerdAddress   = "unix:///var/run/containerd/containerd.sock"
	DefaultDataRoot            = "/var/lib/containerd"
	DefaultRuntime             = "devbox-runc"
	InsecureRegistry           = true
	PauseContainerDuringCommit = false

	AnnotationKeyNamespace               = "namespace"
	AnnotationKeyImageName               = "image.name"
	DevboxOptionsRemoveBaseImageTopLayer = true
	AnnotationImageFromValue             = "true"
	AnnotationUseLimitValue              = "1Gi"
)
