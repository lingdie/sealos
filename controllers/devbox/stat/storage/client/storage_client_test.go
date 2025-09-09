package storageclient

import (
	"context"
	"testing"

	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

func TestStorageClientInSidecar(t *testing.T) {
	config := DefaultStorageClientConfig()
	client, err := NewStorageClient(config)
	if err != nil {
		t.Errorf("Failed to create storage client: %v", err)
	}
	defer client.Close()

	t.Run("HealthCheck", func(t *testing.T) {
		if err := client.HealthCheck(context.Background()); err != nil {
			t.Errorf("Health check failed: %v", err)
		}
		logger.Info("gRPC server health check passed")
	})

	t.Run("GetStorageStats", func(t *testing.T) {
		stats, err := client.GetStorageStats(context.Background())
		if err != nil {
			t.Errorf("Get storage stats failed: %v", err)
		}
		logger.Info("storage stats: %v", stats)
	})

	t.Run("GetStorageMetadata", func(t *testing.T) {
		metadata, err := client.GetStorageMetadata(context.Background())
		if err != nil {
			t.Errorf("Get storage metadata failed: %v", err)
		}
		logger.Info("storage metadata: %v", metadata)
	})
}
