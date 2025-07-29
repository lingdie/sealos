package helper

type AcceptanceConsideration struct {
	// The percentage of available bytes required to consider the node suitable for scheduling devbox
	ContainerFSThreshold uint
	// The percentage of available CPU required to consider the node suitable for scheduling devbox
	CPURequestRatio uint
	CPULimitRatio   uint
}
