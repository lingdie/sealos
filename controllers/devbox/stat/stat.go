package stat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/labring/sealos/controllers/pkg/utils/logger"
	"github.com/openebs/lvm-localpv/pkg/apis/openebs.io/lvm/v1alpha1"
	"github.com/openebs/lvm-localpv/pkg/lvm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FsStats contains data about filesystem usage.
// This part of code is taken from k8s.io/kubelet/pkg/apis/stats/v1alpha1
// Maybe we should import it directly in the future.
type FsStats struct {
	// The time at which these stats were updated.
	Time metav1.Time `json:"time"`
	// AvailableBytes represents the storage space available (bytes) for the filesystem.
	// +optional
	AvailableBytes *uint64 `json:"availableBytes,omitempty"`
	// CapacityBytes represents the total capacity (bytes) of the filesystems underlying storage.
	// +optional
	CapacityBytes *uint64 `json:"capacityBytes,omitempty"`
	// UsedBytes represents the bytes used for a specific task on the filesystem.
	// This may differ from the total bytes used on the filesystem and may not equal CapacityBytes - AvailableBytes.
	// e.g. For ContainerStats.Rootfs this is the bytes used by the container rootfs on the filesystem.
	// +optional
	UsedBytes *uint64 `json:"usedBytes,omitempty"`
	// InodesFree represents the free inodes in the filesystem.
	// +optional
	InodesFree *uint64 `json:"inodesFree,omitempty"`
	// Inodes represents the total inodes in the filesystem.
	// +optional
	Inodes *uint64 `json:"inodes,omitempty"`
	// InodesUsed represents the inodes used by the filesystem
	// This may not equal Inodes - InodesFree because this filesystem may share inodes with other "filesystems"
	// e.g. For ContainerStats.Rootfs, this is the inodes used only by that container, and does not count inodes used by other containers.
	InodesUsed *uint64 `json:"inodesUsed,omitempty"`
}

// StorageStats storage stats
type StorageStats struct {
	Timestamp      time.Time              `json:"timestamp"`
	NodeName       string                 `json:"node_name"`
	StorageType    string                 `json:"storage_type"` // "lvm", "ceph", "nfs" ...
	CapacityBytes  *uint64                `json:"capacityBytes,omitempty"`
	AvailableBytes *uint64                `json:"availableBytes,omitempty"`
	UsedBytes      *uint64                `json:"usedBytes,omitempty"`
	InodesFree     *uint64                `json:"inodesFree,omitempty"`
	Inodes         *uint64                `json:"inodes,omitempty"`
	InodesUsed     *uint64                `json:"inodesUsed,omitempty"`
	DataPercent    float64                `json:"data_percent,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"` // other metrics
}

// StorageMetadata storage metadata
type StorageMetadata struct {
	Timestamp              time.Time              `json:"timestamp"`
	StorageType            string                 `json:"storage_type"` // "lvm", "ceph", "nfs" ...
	NodeName               string                 `json:"node_name"`
	MetadataCapacityBytes  *uint64                `json:"metadata_capacityBytes,omitempty"`
	MetadataAvailableBytes *uint64                `json:"metadata_availableBytes,omitempty"`
	MetadataUsedBytes      *uint64                `json:"metadata_usedBytes,omitempty"`
	MetadataUsedPercent    float64                `json:"metadata_used_percent,omitempty"`
	Metrics                map[string]interface{} `json:"metrics,omitempty"` // other metrics
}

// ThinPoolMetrics contains detailed thin pool metrics for monitoring
type ThinPoolMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	ThinPoolName string    `json:"thin_pool_name"`
	NodeName     string    `json:"node_name"`
	VGName       string    `json:"vg_name"`
	UUID         string    `json:"uuid"`
	HealthStatus int       `json:"health_status"`
	ActiveStatus string    `json:"active_status"`
	PoolName     string    `json:"pool_name"`

	// Capacity metrics
	TotalSize        int64 `json:"total_size_bytes"`
	UsedSize         int64 `json:"used_size_bytes"`
	VGFreeSize       int64 `json:"vg_free_size_bytes"`
	MetadataSize     int64 `json:"metadata_size_bytes"`
	MetadataUsedSize int64 `json:"metadata_used_bytes"`
	MetadataFreeSize int64 `json:"metadata_free_bytes"`

	// Usage metrics
	DataPercent         float64 `json:"data_percent"`
	MetadataUsedPercent float64 `json:"metadata_used_percent"`

	// TODO: add snapshot metrics
	// Snapshot metrics
	SnapshotUsedPercent float64 `json:"snapshot_used_percent"`
}

// NodeStatsProvider storage backend independent interface
type NodeStatsProvider interface {
	GetStorageStats(ctx context.Context) (*StorageStats, error)
	GetStorageMetadata(ctx context.Context) (*StorageMetadata, error)
}

// NewNodeStatsProvider create node stats provider
func NewNodeStatsProvider(storageType string) NodeStatsProvider {
	switch storageType {
	case StorageTypeLVM:
		return NewLVMStatsProvider()
	default:
		return NewLVMStatsProvider()
	}
}

// LVMStatsProvider LVM storage backend implementation
type LVMStatsProvider struct {
	storageType string
}

// NewLVMStatsProvider create LVM storage backend implementation
func NewLVMStatsProvider() *LVMStatsProvider {
	return &LVMStatsProvider{storageType: StorageTypeLVM}
}

// GetStorageStats get storage stats
func (l *LVMStatsProvider) GetStorageStats(ctx context.Context) (*StorageStats, error) {
	// get thin pool metrics
	thinPoolMetrics, err := l.collectThinPoolMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect thin pool metrics: %w", err)
	}

	if len(thinPoolMetrics) == 0 {
		return nil, fmt.Errorf("no thin pool found")
	}

	// use the first thin pool
	metrics := thinPoolMetrics[0]

	// use the logical capacity of the thin pool, not the device file capacity
	capacityBytes := uint64(metrics.TotalSize)
	usedBytes := uint64(metrics.UsedSize)
	availableBytes := capacityBytes - usedBytes

	// for inode, thin pool may not have a direct concept
	// use default value or get from file system
	var totalInodes, freeInodes, usedInodes uint64

	// try to get inode information from the thin pool device
	vgName := strings.ReplaceAll(metrics.VGName, "-", "--")
	thinPoolName := strings.ReplaceAll(metrics.ThinPoolName, "-", "--")
	thinPoolPath := fmt.Sprintf("/dev/mapper/%s-%s", vgName, thinPoolName)

	var stat syscall.Statfs_t
	if err := syscall.Statfs(thinPoolPath, &stat); err == nil {
		totalInodes = stat.Files
		freeInodes = stat.Ffree
		usedInodes = totalInodes - freeInodes
	} else {
		// if cannot get inode information, use default value
		totalInodes = 1000000
		freeInodes = 800000
		usedInodes = 200000
	}

	return &StorageStats{
		Timestamp:      metrics.Timestamp,
		NodeName:       metrics.NodeName,
		StorageType:    l.storageType,
		CapacityBytes:  &capacityBytes,
		AvailableBytes: &availableBytes,
		UsedBytes:      &usedBytes,
		Inodes:         &totalInodes,
		InodesFree:     &freeInodes,
		InodesUsed:     &usedInodes,
		DataPercent:    metrics.DataPercent,
		Metrics:        map[string]interface{}{"thinpool": metrics},
	}, nil
}

// GetStorageMetadata
func (l *LVMStatsProvider) GetStorageMetadata(ctx context.Context) (*StorageMetadata, error) {
	thinPoolMetrics, err := l.collectThinPoolMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect thin pool metrics: %w", err)
	}

	metrics := thinPoolMetrics[0]

	metadataCapacityBytes := uint64(metrics.MetadataSize)
	metadataAvailableBytes := uint64(metrics.MetadataFreeSize)
	metadataUsedBytes := uint64(metrics.MetadataUsedSize)
	metadataUsedPercent := metrics.MetadataUsedPercent

	return &StorageMetadata{
		Timestamp:              metrics.Timestamp,
		StorageType:            l.storageType,
		NodeName:               metrics.NodeName,
		MetadataCapacityBytes:  &metadataCapacityBytes,
		MetadataAvailableBytes: &metadataAvailableBytes,
		MetadataUsedBytes:      &metadataUsedBytes,
		MetadataUsedPercent:    metadataUsedPercent,
		Metrics:                map[string]interface{}{"thinpool": metrics},
	}, nil
}

// collectThinPoolMetrics collect thin pool metrics
func (l *LVMStatsProvider) collectThinPoolMetrics() ([]*ThinPoolMetrics, error) {
	// refresh lvm metadata cache
	if err := lvm.ReloadLVMMetadataCache(); err != nil {
		return nil, fmt.Errorf("failed to reload LVM metadata cache: %w", err)
	}

	// get all logical volumes
	lvs, err := lvm.ListLVMLogicalVolume()
	if err != nil {
		return nil, fmt.Errorf("failed to list LVM logical volumes: %w", err)
	}

	// get all volume groups
	vgs, err := lvm.ListLVMVolumeGroup(false)
	if err != nil {
		return nil, fmt.Errorf("failed to list LVM volume groups: %w", err)
	}

	// create volume group mapping
	vgMap := make(map[string]v1alpha1.VolumeGroup)
	for _, vg := range vgs {
		vgMap[vg.Name] = vg
	}

	var thinPools []*ThinPoolMetrics
	now := time.Now()

	for _, lv := range lvs {
		// only process thin pool type logical volumes
		if lv.SegType == lvm.LVThinPool {
			metrics := &ThinPoolMetrics{
				ThinPoolName:        lv.Name,
				VGName:              lv.VGName,
				UUID:                lv.UUID,                // UUID
				TotalSize:           lv.Size,                // total size
				DataPercent:         lv.UsedSizePercent,     // data usage percentage
				MetadataSize:        lv.MetadataSize,        // metadata size
				MetadataUsedPercent: lv.MetadataUsedPercent, // metadata usage percentage
				HealthStatus:        lv.HealthStatus,        // health status
				ActiveStatus:        lv.ActiveStatus,        // active status
				NodeName:            lv.Host,                // node name
				PoolName:            lv.PoolName,            // pool name
				SnapshotUsedPercent: lv.SnapshotUsedPercent, // snapshot usage percentage
				Timestamp:           now,                    // current time
			}

			// get available space from volume group
			if vg, exists := vgMap[lv.VGName]; exists {
				metrics.VGFreeSize = vg.Free.Value()
				metrics.UsedSize = int64(float64(metrics.TotalSize) * metrics.DataPercent / 100)

				// calculate metadata remaining space
				if metrics.MetadataSize > 0 {
					metrics.MetadataUsedSize = int64(float64(metrics.MetadataSize) * metrics.MetadataUsedPercent / 100)
					metrics.MetadataFreeSize = metrics.MetadataSize - metrics.MetadataUsedSize
				}
			}

			thinPools = append(thinPools, metrics)
		}
	}

	return thinPools, nil
}

// ExportToVictoriaMetrics call exportToVictoriaMetrics to exports the thin pool metrics to VictoriaMetrics
func (l *LVMStatsProvider) ExportToVictoriaMetrics(metrics []*ThinPoolMetrics) error {
	return l.exportToVictoriaMetrics(metrics)
}

// exportToVictoriaMetrics exports the thin pool metrics to VictoriaMetrics
func (l *LVMStatsProvider) exportToVictoriaMetrics(metricsSlice []*ThinPoolMetrics) error {
	// build all thin pool prometheus format data
	var allPrometheusData []string

	for _, metrics := range metricsSlice {
		// build metrics data for each thin pool
		prometheusData := []string{
			fmt.Sprintf("thin_pool_data_usage_percent{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %f %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.DataPercent, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_metadata_usage_percent{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %f %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.MetadataUsedPercent, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_snapshot_usage_percent{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %f %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.SnapshotUsedPercent, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_total_size_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.TotalSize, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_used_size_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.UsedSize, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_vg_free_size_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.VGFreeSize, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_metadata_size_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.MetadataSize, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_metadata_used_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.MetadataUsedSize, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_metadata_free_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.MetadataFreeSize, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_snapshot_count{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.SnapshotCount, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_snapshot_size_bytes{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.SnapshotSize, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_read_iops{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.ReadIOPS, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_write_iops{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.WriteIOPS, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_read_latency_ms{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %f %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.ReadLatency, metrics.Timestamp.Unix()),

			// fmt.Sprintf("thin_pool_write_latency_ms{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %f %d",
			// 	metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.WriteLatency, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_health_status{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, metrics.HealthStatus, metrics.Timestamp.Unix()),

			fmt.Sprintf("thin_pool_active_status{thin_pool=\"%s\",node=\"%s\",vg=\"%s\",storage_type=\"lvm\"} %d %d",
				metrics.ThinPoolName, metrics.NodeName, metrics.VGName, boolToInt(metrics.ActiveStatus == "active"), metrics.Timestamp.Unix()),
		}

		// add current thin pool metrics to total list
		allPrometheusData = append(allPrometheusData, prometheusData...)
	}

	// join data into string
	data := strings.Join(allPrometheusData, "\n")
	// todo : delete data and save length
	logger.Info("[exportToVictoriaMetrics] Sending %d metrics to VM:\n%s\n", len(allPrometheusData), data)

	// use VM insert endpoint
	logger.Info("[exportToVictoriaMetrics] Sending to URL: %s\n", VMImportURL)

	// use text/plain content type
	resp, err := http.Post(VMImportURL, "text/plain", strings.NewReader(data))
	if err != nil {
		logger.Error("[exportToVictoriaMetrics] HTTP request failed: %v\n", err)
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// read response content
	body, _ := io.ReadAll(resp.Body)
	logger.Info("[exportToVictoriaMetrics] VM response status: %d, body: %s\n", resp.StatusCode, string(body))

	// check response status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("VM returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	logger.Info("[exportToVictoriaMetrics] Successfully sent data to VM (status: %d)\n", resp.StatusCode)
	return nil
}

// boolToInt converts a boolean to an integer (1 for true, 0 for false)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
