package lvmmonitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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

	logger.Info("gRPC server test completed successfully")
}
