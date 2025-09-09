package storageclient

import (
	"context"
	"fmt"
	"time"

	"github.com/labring/sealos/controllers/devbox/stat"
	pb "github.com/labring/sealos/controllers/devbox/stat/proto"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type StorageClient struct {
	conn         *grpc.ClientConn
	client       pb.StorageServiceClient
	healthClient grpc_health_v1.HealthClient
	address      string
	timeout      time.Duration
}

type StorageClientConfig struct {
	Address       string
	Timeout       time.Duration
	RetryCount    int
	RetryInterval time.Duration
}

func DefaultStorageClientConfig() *StorageClientConfig {
	return &StorageClientConfig{
		Address:       stat.DefaultStorageClientAddr,
		Timeout:       stat.DefaultStorageClientTimeout,
		RetryCount:    stat.DefaultStorageClientRetryCount,
		RetryInterval: stat.DefaultStorageClientRetryInterval,
	}
}

func NewStorageClient(config *StorageClientConfig) (*StorageClient, error) {
	if config == nil {
		config = DefaultStorageClientConfig()
	}

	conn, err := grpc.NewClient(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	client := &StorageClient{
		conn:         conn,
		client:       pb.NewStorageServiceClient(conn),
		healthClient: grpc_health_v1.NewHealthClient(conn),
		address:      config.Address,
		timeout:      config.Timeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := client.HealthCheck(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	logger.Info("Storage client connected successfully")

	return client, nil
}

// Close closes the LVM client connection
func (c *StorageClient) Close() error {
	if c.conn != nil {
		logger.Info("Closing Storage client connection")
		return c.conn.Close()
	}
	return nil
}

// HealthCheck checks the health status of the Storage client
func (c *StorageClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("service not healthy: %v", resp.Status)
	}

	return nil
}

func (c *StorageClient) GetStorageStats(ctx context.Context) (*pb.StorageStatsProto, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetStorageStats(ctx, &pb.GetStorageStatsRequest{})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}

	return resp.Stats, nil
}

func (c *StorageClient) GetStorageMetadata(ctx context.Context) (*pb.StorageMetadataProto, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetStorageMetadata(ctx, &pb.GetStorageMetadataRequest{})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}

	return resp.Metadata, nil
}
