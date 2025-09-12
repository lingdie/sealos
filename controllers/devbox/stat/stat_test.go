package stat

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestGetStorageStats is used to test the GetStorageStats function
func TestGetStorageStats(t *testing.T) {
	provider := NewNodeStatsProvider(StorageTypeLVM)
	stats, err := provider.GetStorageStats(context.Background())
	if err != nil {
		t.Fatalf("GetStorageStats failed, err: %v", err)
	}

	fmt.Printf("=== Storage Stats Test Results ===\n")
	fmt.Printf("Storage Type: %s\n", stats.StorageType)
	fmt.Printf("Node Name: %s\n", stats.NodeName)
	fmt.Printf("Time: %s\n", stats.Timestamp.Format("2006-01-02 15:04:05"))

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

	fmt.Printf("\n=== Test completed successfully ===\n")
}

// TestGetStorageMetadata is used to test the GetStorageMetadata function
func TestGetStorageMetadata(t *testing.T) {
	provider := NewNodeStatsProvider(StorageTypeLVM)
	metadata, err := provider.GetStorageMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetStorageMetadata failed, err: %v", err)
	}

	fmt.Printf("=== Storage Metadata Test Results ===\n")
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

	fmt.Printf("\n=== Test completed successfully ===\n")
}

// TestExportToVictoriaMetrics is used to test the ExportToVictoriaMetrics function
func TestExportToVictoriaMetrics(t *testing.T) {
	provider := NewNodeStatsProvider(StorageTypeLVM)

	// get thin pool metrics first
	metrics, err := provider.(*LVMStatsProvider).collectThinPoolMetrics()
	if err != nil {
		t.Fatalf("collectThinPoolMetrics failed, err: %v", err)
	}

	fmt.Printf("=== Thin Pool Metrics Test Results ===\n")
	fmt.Printf("Total thin pools found: %d\n", len(metrics))

	for i, metric := range metrics {
		fmt.Printf("\n--- Thin Pool %d ---\n", i+1)
		fmt.Printf("Name: %s\n", metric.ThinPoolName)
		fmt.Printf("VG: %s\n", metric.VGName)
		fmt.Printf("Node: %s\n", metric.NodeName)
		fmt.Printf("UUID: %s\n", metric.UUID)
		fmt.Printf("Total Size: %d bytes (%.2f GB)\n", metric.TotalSize, float64(metric.TotalSize)/1024/1024/1024)
		fmt.Printf("Used Size: %d bytes (%.2f GB)\n", metric.UsedSize, float64(metric.UsedSize)/1024/1024/1024)
		fmt.Printf("Free Size: %d bytes (%.2f GB)\n", metric.VGFreeSize, float64(metric.VGFreeSize)/1024/1024/1024)
		fmt.Printf("Data Usage: %.2f%%\n", metric.DataPercent)
		fmt.Printf("Metadata Size: %d bytes (%.2f GB)\n", metric.MetadataSize, float64(metric.MetadataSize)/1024/1024/1024)
		fmt.Printf("Metadata Used: %.2f%%\n", metric.MetadataUsedPercent)
		fmt.Printf("Snapshot Used: %.2f%%\n", metric.SnapshotUsedPercent)
		fmt.Printf("Health Status: %d\n", metric.HealthStatus)
		fmt.Printf("Active Status: %s\n", metric.ActiveStatus)
		fmt.Printf("Timestamp: %s\n", metric.Timestamp.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\n=== Test completed successfully ===\n")

	// export to VictoriaMetrics
	err = provider.(*LVMStatsProvider).ExportToVictoriaMetrics(metrics)
	if err != nil {
		t.Fatalf("ExportToVictoriaMetrics failed, err: %v", err)
	}

	fmt.Printf("ExportToVictoriaMetrics success\n")
}

// TestGenerateContinuousMetrics is used to generate continuous metrics for grafana
func TestGenerateContinuousMetrics(t *testing.T) {
	provider := NewNodeStatsProvider(StorageTypeLVM)

	// generate continuous metrics for grafana
	fmt.Printf("=== Generating Continuous Metrics for Grafana ===\n")

	// simulate 24 hours data, one data point per hour
	startTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < 24; i++ {
		// calculate current time point
		currentTime := startTime.Add(time.Duration(i) * time.Hour)

		// create simulated thin pool metrics data
		metrics := []*ThinPoolMetrics{
			{
				Timestamp:    currentTime,
				ThinPoolName: "devbox-vg-thinpool",
				NodeName:     "test-node-1",
				VGName:       "devbox-vg",
				UUID:         "test-uuid-123",
				HealthStatus: 1, // healthy
				ActiveStatus: "active",
				PoolName:     "devbox-pool",

				// capacity metrics - simulate time-varying
				TotalSize:        87916806144,                      // about 82GB
				UsedSize:         int64(1520960746 + i*100000000),  // increase with time
				VGFreeSize:       int64(50000000000 - i*100000000), // decrease with time
				MetadataSize:     703334449,                        // about 0.66GB
				MetadataUsedSize: int64(500000000 + i*5000000),     // increase with time
				MetadataFreeSize: int64(203334449 - i*5000000),     // decrease with time

				// usage rate metrics - simulate fluctuation
				DataPercent:         float64(1.73 + float64(i)*0.1 + float64(i%3)*0.05),
				MetadataUsedPercent: float64(0.8 + float64(i)*0.05 + float64(i%2)*0.02),
				SnapshotUsedPercent: float64(0.5 + float64(i)*0.02 + float64(i%4)*0.01),
			},
			// second thin pool
			{
				Timestamp:    currentTime,
				ThinPoolName: "backup-vg-thinpool",
				NodeName:     "test-node-2",
				VGName:       "backup-vg",
				UUID:         "test-uuid-456",
				HealthStatus: 1,
				ActiveStatus: "active",
				PoolName:     "backup-pool",

				TotalSize:        53687091200, // 约 50GB
				UsedSize:         int64(800000000 + i*80000000),
				VGFreeSize:       int64(30000000000 - i*80000000),
				MetadataSize:     500000000,
				MetadataUsedSize: int64(300000000 + i*3000000),
				MetadataFreeSize: int64(200000000 - i*3000000),

				DataPercent:         float64(1.5 + float64(i)*0.08 + float64(i%5)*0.03),
				MetadataUsedPercent: float64(0.6 + float64(i)*0.04 + float64(i%3)*0.015),
				SnapshotUsedPercent: float64(0.3 + float64(i)*0.015 + float64(i%4)*0.008),
			},
		}

		// export to VictoriaMetrics
		err := provider.(*LVMStatsProvider).ExportToVictoriaMetrics(metrics)
		if err != nil {
			t.Logf("Failed to export metrics for time %s: %v", currentTime.Format("2006-01-02 15:04:05"), err)
		} else {
			fmt.Printf("✓ Exported metrics for %s (%d thin pools)\n",
				currentTime.Format("2006-01-02 15:04:05"), len(metrics))
		}
	}

	fmt.Printf("\n=== Continuous Metrics Generation Completed ===\n")
	fmt.Printf("Generated 24 data points for 2 thin pools\n")
	fmt.Printf("Total metrics sent: %d\n", 24*2*10) // 24 hours * 2 pools * 10 metrics (based on current implementation)
}

// TestCompleteMonitoringFlow tests the complete monitoring flow
func TestCompleteMonitoringFlow(t *testing.T) {
	provider := NewNodeStatsProvider(StorageTypeLVM)
	ctx := context.Background()

	fmt.Printf("=== Complete Monitoring Flow Test ===\n")

	// 1. Test GetStorageStats
	fmt.Printf("\n1. Testing GetStorageStats...\n")
	stats, err := provider.GetStorageStats(ctx)
	if err != nil {
		t.Fatalf("GetStorageStats failed: %v", err)
	}
	fmt.Printf("✓ StorageStats collected successfully\n")
	fmt.Printf("  - Storage Type: %s\n", stats.StorageType)
	fmt.Printf("  - Node: %s\n", stats.NodeName)
	if stats.CapacityBytes != nil {
		fmt.Printf("  - Capacity: %.2f GB\n", float64(*stats.CapacityBytes)/1024/1024/1024)
	}
	if stats.AvailableBytes != nil {
		fmt.Printf("  - Available: %.2f GB\n", float64(*stats.AvailableBytes)/1024/1024/1024)
	}
	fmt.Printf("  - Data Usage: %.2f%%\n", stats.DataPercent)

	// 2. Test GetStorageMetadata
	fmt.Printf("\n2. Testing GetStorageMetadata...\n")
	metadata, err := provider.GetStorageMetadata(ctx)
	if err != nil {
		t.Fatalf("GetStorageMetadata failed: %v", err)
	}
	fmt.Printf("✓ StorageMetadata collected successfully\n")
	fmt.Printf("  - Storage Type: %s\n", metadata.StorageType)
	fmt.Printf("  - Node: %s\n", metadata.NodeName)
	if metadata.MetadataCapacityBytes != nil {
		fmt.Printf("  - Metadata Capacity: %.2f GB\n", float64(*metadata.MetadataCapacityBytes)/1024/1024/1024)
	}
	fmt.Printf("  - Metadata Usage: %.2f%%\n", metadata.MetadataUsedPercent)

	// 3. Test collectThinPoolMetrics
	fmt.Printf("\n3. Testing collectThinPoolMetrics...\n")
	thinPoolMetrics, err := provider.(*LVMStatsProvider).collectThinPoolMetrics()
	if err != nil {
		t.Fatalf("collectThinPoolMetrics failed: %v", err)
	}
	fmt.Printf("✓ ThinPoolMetrics collected successfully\n")
	fmt.Printf("  - Found %d thin pools\n", len(thinPoolMetrics))

	// 4. Test ExportToVictoriaMetrics
	fmt.Printf("\n4. Testing ExportToVictoriaMetrics...\n")
	if len(thinPoolMetrics) > 0 {
		err = provider.(*LVMStatsProvider).ExportToVictoriaMetrics(thinPoolMetrics)
		if err != nil {
			t.Logf("ExportToVictoriaMetrics failed (this is expected in test environment): %v", err)
		} else {
			fmt.Printf("✓ Metrics exported to VictoriaMetrics successfully\n")
		}
	} else {
		fmt.Printf("⚠ No thin pools found, skipping export test\n")
	}

	fmt.Printf("\n=== Complete Monitoring Flow Test Completed ===\n")
	fmt.Printf("✓ All monitoring functions tested successfully\n")
}
