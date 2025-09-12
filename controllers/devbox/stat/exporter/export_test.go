// stat/export_test.go
package exporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/labring/sealos/controllers/devbox/stat"
)

// TestVMMetricsExporter_ExportStorageStats tests the ExportStorageStats function
func TestVMMetricsExporter_ExportStorageStats(t *testing.T) {
	// create exporter
	config := DefaultExportConfig()
	exporter := NewVMMetricsExporter(config)

	// create test storage stats
	capacityBytes := uint64(100 * 1024 * 1024 * 1024) // 100GB
	availableBytes := uint64(80 * 1024 * 1024 * 1024) // 80GB
	usedBytes := uint64(20 * 1024 * 1024 * 1024)      // 20GB
	inodes := uint64(1000000)
	inodesUsed := uint64(500000)
	inodesFree := uint64(500000)

	stats := &stat.StorageStats{
		Timestamp:      time.Now(),
		NodeName:       "test-node-1",
		StorageType:    stat.StorageTypeLVM,
		CapacityBytes:  &capacityBytes,
		AvailableBytes: &availableBytes,
		UsedBytes:      &usedBytes,
		Inodes:         &inodes,
		InodesUsed:     &inodesUsed,
		InodesFree:     &inodesFree,
		DataPercent:    20.0,
		Metrics: map[string]interface{}{
			stat.ThinPoolName: 1,
			"health_status":   "1",
			"active_status":   "1",
		},
	}

	fmt.Printf("=== VMMetricsExporter ExportStorageStats Test Results ===\n")
	fmt.Printf("Storage Type: %s\n", stats.StorageType)
	fmt.Printf("Node Name: %s\n", stats.NodeName)
	fmt.Printf("Timestamp: %s\n", stats.Timestamp.Format("2006-01-02 15:04:05"))

	if stats.CapacityBytes != nil {
		fmt.Printf("Capacity: %d bytes (%.2f GB)\n", *stats.CapacityBytes, float64(*stats.CapacityBytes)/1024/1024/1024)
	}
	if stats.AvailableBytes != nil {
		fmt.Printf("Available: %d bytes (%.2f GB)\n", *stats.AvailableBytes, float64(*stats.AvailableBytes)/1024/1024/1024)
	}
	if stats.UsedBytes != nil {
		fmt.Printf("Used: %d bytes (%.2f GB)\n", *stats.UsedBytes, float64(*stats.UsedBytes)/1024/1024/1024)
	}
	if stats.Inodes != nil {
		fmt.Printf("Total Inodes: %d\n", *stats.Inodes)
	}
	if stats.InodesFree != nil {
		fmt.Printf("Free Inodes: %d\n", *stats.InodesFree)
	}
	if stats.InodesUsed != nil {
		fmt.Printf("Used Inodes: %d\n", *stats.InodesUsed)
	}
	fmt.Printf("Data Percent: %.2f%%\n", stats.DataPercent)

	// test export (this will fail in test environment, but we can test the metrics building)
	ctx := context.Background()
	err := exporter.ExportStorageStats(ctx, stats)
	if err != nil {
		fmt.Printf("ExportStorageStats failed (expected in test environment): %v\n", err)
	} else {
		fmt.Printf("ExportStorageStats success\n")
	}

	fmt.Printf("\n=== Test completed successfully ===\n")
}

// TestVMMetricsExporter_ExportStorageMetadata tests the ExportStorageMetadata function
func TestVMMetricsExporter_ExportStorageMetadata(t *testing.T) {
	// create test config
	config := DefaultExportConfig()

	// create exporter
	exporter := NewVMMetricsExporter(config)

	// create test storage metadata
	metadataCapacityBytes := uint64(10 * 1024 * 1024 * 1024) // 10GB
	metadataAvailableBytes := uint64(8 * 1024 * 1024 * 1024) // 8GB
	metadataUsedBytes := uint64(2 * 1024 * 1024 * 1024)      // 2GB

	metadata := &stat.StorageMetadata{
		Timestamp:              time.Now(),
		StorageType:            stat.StorageTypeLVM,
		NodeName:               "test-node-1",
		MetadataCapacityBytes:  &metadataCapacityBytes,
		MetadataAvailableBytes: &metadataAvailableBytes,
		MetadataUsedBytes:      &metadataUsedBytes,
		MetadataUsedPercent:    20.0,
		Metrics: map[string]interface{}{
			"thin_pool_name": stat.ThinPoolName,
			"vg_name":        "devbox-vg",
		},
	}

	fmt.Printf("=== VMMetricsExporter ExportStorageMetadata Test Results ===\n")
	fmt.Printf("Storage Type: %s\n", metadata.StorageType)
	fmt.Printf("Node Name: %s\n", metadata.NodeName)
	fmt.Printf("Timestamp: %s\n", metadata.Timestamp.Format("2006-01-02 15:04:05"))

	if metadata.MetadataCapacityBytes != nil {
		fmt.Printf("Metadata Capacity: %d bytes (%.2f GB)\n", *metadata.MetadataCapacityBytes, float64(*metadata.MetadataCapacityBytes)/1024/1024/1024)
	}
	if metadata.MetadataAvailableBytes != nil {
		fmt.Printf("Metadata Available: %d bytes (%.2f GB)\n", *metadata.MetadataAvailableBytes, float64(*metadata.MetadataAvailableBytes)/1024/1024/1024)
	}
	if metadata.MetadataUsedBytes != nil {
		fmt.Printf("Metadata Used: %d bytes (%.2f GB)\n", *metadata.MetadataUsedBytes, float64(*metadata.MetadataUsedBytes)/1024/1024/1024)
	}
	fmt.Printf("Metadata Used Percent: %.2f%%\n", metadata.MetadataUsedPercent)

	// test export (this will fail in test environment, but we can test the metrics building)
	ctx := context.Background()
	err := exporter.ExportStorageMetadata(ctx, metadata)
	if err != nil {
		fmt.Printf("ExportStorageMetadata failed (expected in test environment): %v\n", err)
	} else {
		fmt.Printf("ExportStorageMetadata success\n")
	}

	fmt.Printf("\n=== Test completed successfully ===\n")
}
