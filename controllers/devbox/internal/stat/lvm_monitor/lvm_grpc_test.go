package lvmmonitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/labring/sealos/controllers/devbox/internal/stat"
	lvmmonitorclient "github.com/labring/sealos/controllers/devbox/internal/stat/lvm_monitor/lvm_monitor_client"
	lvmmonitorserver "github.com/labring/sealos/controllers/devbox/internal/stat/lvm_monitor/lvm_monitor_server"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

// TestGRPCServerAvailability is used to test the gRPC server availability
func TestGRPCServerAvailability(t *testing.T) {
	// test configuration
	grpcPort := 9091
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start gRPC server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		lvmmonitorserver.StartLVMMonitorServer(ctx, grpcPort)
	}()

	// wait for server to start
	time.Sleep(2 * time.Second)

	// use LVM client to test
	clientConfig := &lvmmonitorclient.LVMClientConfig{
		Address:       fmt.Sprintf("localhost:%d", grpcPort),
		Timeout:       5 * time.Second,
		RetryCount:    3,
		RetryInterval: 1 * time.Second,
	}

	client, err := lvmmonitorclient.NewLVMClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create LVM client: %v", err)
	}
	defer client.Close()

	// test health check
	t.Run("HealthCheck", func(t *testing.T) {
		if err := client.HealthCheck(context.Background()); err != nil {
			t.Errorf("Health check failed: %v", err)
		}
		logger.Info("gRPC server health check passed")
	})

	// test get thin pool metrics
	t.Run("GetThinPoolMetrics", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics failed: %v", err)
		}

		if metrics == nil {
			t.Error("Expected metrics, got nil")
		}

		logger.Info("LVM monitor service test passed, got %d metrics", len(metrics))
	})

	// test get thin pool metrics by VG name
	t.Run("GetThinPoolMetricsByVG", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{
			VGName: "devbox-vg",
		}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics by VG failed: %v", err)
		}

		logger.Info("GetThinPoolMetrics by VG passed, got %d metrics", len(metrics))
	})

	// test get thin pool metrics by thin pool name
	t.Run("GetThinPoolMetricsByName", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{
			ThinPoolName: "devbox-vg-thinpool",
		}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics by name failed: %v", err)
		}

		logger.Info("GetThinPoolMetrics by name passed, got %d metrics", len(metrics))
	})

	// test get container filesystem stats
	t.Run("GetContainerFsStats", func(t *testing.T) {
		stats, err := client.GetContainerFsStats(context.Background())
		if err != nil {
			t.Errorf("GetContainerFsStats failed: %v", err)
		}
		logger.Info("GetContainerFsStats passed, got %d stats", stats)
	})

	// stop server
	cancel()
	wg.Wait()

	logger.Info("gRPC server test completed successfully")
}

func TestGRPCClientInSidecar(t *testing.T) {
	// test configuration
	grpcPort := 9090

	// use LVM client to test
	clientConfig := &lvmmonitorclient.LVMClientConfig{
		Address:       fmt.Sprintf("localhost:%d", grpcPort),
		Timeout:       5 * time.Second,
		RetryCount:    3,
		RetryInterval: 1 * time.Second,
	}

	client, err := lvmmonitorclient.NewLVMClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create LVM client: %v", err)
	}
	defer client.Close()

	// test health check
	t.Run("HealthCheck", func(t *testing.T) {
		if err := client.HealthCheck(context.Background()); err != nil {
			t.Errorf("Health check failed: %v", err)
		}
		logger.Info("gRPC server health check passed")
	})

	// test get thin pool metrics
	t.Run("GetThinPoolMetrics", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics failed: %v", err)
		}

		if metrics == nil {
			t.Error("Expected metrics, got nil")
		}

		logger.Info("LVM monitor service test passed, got %d metrics", len(metrics))
	})

	// test get thin pool metrics by VG name
	t.Run("GetThinPoolMetricsByVG", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{
			VGName: "devbox-vg",
		}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics by VG failed: %v", err)
		}

		logger.Info("GetThinPoolMetrics by VG passed, got %d metrics", len(metrics))
	})

	// test get thin pool metrics by thin pool name
	t.Run("GetThinPoolMetricsByName", func(t *testing.T) {
		options := &lvmmonitorclient.ThinPoolQueryOptions{
			ThinPoolName: "devbox-vg-thinpool",
		}
		metrics, err := client.GetThinPoolMetrics(context.Background(), options)
		if err != nil {
			t.Errorf("GetThinPoolMetrics by name failed: %v", err)
		}

		logger.Info("GetThinPoolMetrics by name passed, got %d metrics", len(metrics))
	})

	// test get container filesystem stats
	t.Run("GetContainerFsStats", func(t *testing.T) {
		stats, err := client.GetContainerFsStats(context.Background())
		if err != nil {
			t.Errorf("GetContainerFsStats failed: %v", err)
		}
		logger.Info("Thin pool logical filesystem stats:")
		logger.Info("  Capacity: %d MB (%d bytes)", *stats.CapacityBytes/1024/1024, *stats.CapacityBytes)
		logger.Info("  Used: %d MB (%d bytes)", *stats.UsedBytes/1024/1024, *stats.UsedBytes)
		logger.Info("  Available: %d MB (%d bytes)", *stats.AvailableBytes/1024/1024, *stats.AvailableBytes)
		logger.Info("  Usage: %.2f%%", float64(*stats.UsedBytes)/float64(*stats.CapacityBytes)*100)
		logger.Info("  Inodes: %d", *stats.Inodes)
		logger.Info("  Inodes Free: %d", *stats.InodesFree)
		logger.Info("  Inodes Used: %d", *stats.InodesUsed)
	})

	logger.Info("gRPC server test completed successfully")
}

func TestContainerFSStats(t *testing.T) {
	nodeImpl := &stat.NodeStatsProviderImpl{}
	stats, err := nodeImpl.ContainerFsStats(context.Background())
	if err != nil {
		t.Fatalf("Failed to get container filesystem stats: %v", err)
	}
	logger.Info("Thin pool logical filesystem stats:")
	logger.Info("  Capacity: %d MB (%d bytes)", *stats.CapacityBytes/1024/1024, *stats.CapacityBytes)
	logger.Info("  Used: %d MB (%d bytes)", *stats.UsedBytes/1024/1024, *stats.UsedBytes)
	logger.Info("  Available: %d MB (%d bytes)", *stats.AvailableBytes/1024/1024, *stats.AvailableBytes)
	logger.Info("  Usage: %.2f%%", float64(*stats.UsedBytes)/float64(*stats.CapacityBytes)*100)
	logger.Info("  Inodes: %d", *stats.Inodes)
	logger.Info("  Inodes Free: %d", *stats.InodesFree)
	logger.Info("  Inodes Used: %d", *stats.InodesUsed)
}
