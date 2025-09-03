package stat

import (
	"fmt"
	"testing"
	"time"
)

// TestCollectThinPoolMetrics is used to test the collectThinPoolMetrics function
func TestCollectThinPoolMetrics(t *testing.T) {
	provider := &NodeStatsProviderImpl{}
	metrics, err := provider.ThinPoolMetrics()
	if err != nil {
		t.Fatalf("ThinPoolMetrics failed, err: %v", err)
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
}

// TestExportToVictoriaMetrics is used to test the exportToVictoriaMetrics function
func TestExportToVictoriaMetrics(t *testing.T) {
	provider := &NodeStatsProviderImpl{}
	metrics, err := provider.ThinPoolMetrics()
	if err != nil {
		t.Fatalf("ThinPoolMetrics failed, err: %v", err)
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

	err = provider.exportToVictoriaMetrics(metrics)
	if err != nil {
		t.Fatalf("exportToVictoriaMetrics failed, err: %v", err)
	}

	fmt.Printf("exportToVictoriaMetrics success\n")
}

// TestGenerateContinuousMetrics is used to generate continuous metrics for grafana
func TestGenerateContinuousMetrics(t *testing.T) {
	provider := &NodeStatsProviderImpl{}

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
				DataPercent:         float64(1.73 + float64(i)*0.1 + float64(i%3)*0.05), // 波动增长
				MetadataUsedPercent: float64(0.8 + float64(i)*0.05 + float64(i%2)*0.02), // 波动增长
				SnapshotUsedPercent: float64(0.5 + float64(i)*0.02 + float64(i%4)*0.01), // 波动增长

				// // snapshot metrics
				// SnapshotCount:       int(10 + i), // increase with time
				// SnapshotSize:        uint64(1000000000 + uint64(i)*50000000), // increase with time

				// // performance metrics - simulate workload change
				// ReadIOPS:            int64(1000 + i*50 + int64(i%6)*100), // simulate workload fluctuation
				// WriteIOPS:           int64(500 + i*30 + int64(i%4)*80),  // simulate workload fluctuation
				// ReadLatency:         float64(5.0 + float64(i)*0.2 + float64(i%3)*0.5), // simulate latency fluctuation
				// WriteLatency:        float64(8.0 + float64(i)*0.3 + float64(i%2)*0.8), // simulate latency fluctuation
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

				// SnapshotCount:       int(5 + i/2),
				// SnapshotSize:        uint64(500000000 + uint64(i)*25000000),

				// ReadIOPS:            int64(800 + i*40 + int64(i%7)*90),
				// WriteIOPS:           int64(400 + i*25 + int64(i%5)*70),
				// ReadLatency:         float64(4.5 + float64(i)*0.18 + float64(i%4)*0.4),
				// WriteLatency:        float64(7.5 + float64(i)*0.25 + float64(i%3)*0.7),
			},
		}

		// export to VictoriaMetrics
		err := provider.exportToVictoriaMetrics(metrics)
		if err != nil {
			t.Logf("Failed to export metrics for time %s: %v", currentTime.Format("2006-01-02 15:04:05"), err)
		} else {
			fmt.Printf("✓ Exported metrics for %s (%d thin pools)\n",
				currentTime.Format("2006-01-02 15:04:05"), len(metrics))
		}

		// // add some delay to avoid sending data too fast
		// time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("\n=== Continuous Metrics Generation Completed ===\n")
	fmt.Printf("Generated 24 data points for 2 thin pools\n")
	fmt.Printf("Total metrics sent: %d\n", 24*2*17) // 24 hours * 2 pools * 17 metrics
}
