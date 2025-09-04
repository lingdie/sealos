package lvmmonitorclient

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/labring/sealos/controllers/devbox/internal/stat"
	pb "github.com/labring/sealos/controllers/devbox/internal/stat/protoc"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type LVMClient struct {
	conn         *grpc.ClientConn
	client       pb.LVMMonitorServiceClient
	healthClient grpc_health_v1.HealthClient
	address      string
	timeout      time.Duration
}

type LVMClientConfig struct {
	Address       string
	Timeout       time.Duration
	RetryCount    int
	RetryInterval time.Duration
}

func DefaultLVMClientConfig() *LVMClientConfig {
	return &LVMClientConfig{
		Address:       stat.DefaultLVMClientAddr,
		Timeout:       stat.DefaultLVMClientTimeout,
		RetryCount:    stat.DefaultLVMClientRetryCount,
		RetryInterval: stat.DefaultLVMClientRetryInterval,
	}
}

// NewLVMClient creates a new LVM client
func NewLVMClient(config *LVMClientConfig) (*LVMClient, error) {
	if config == nil {
		config = DefaultLVMClientConfig()
	}

	conn, err := grpc.NewClient(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithTimeout(config.Timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	client := &LVMClient{
		conn:         conn,
		client:       pb.NewLVMMonitorServiceClient(conn),
		healthClient: grpc_health_v1.NewHealthClient(conn),
		address:      config.Address,
		timeout:      config.Timeout,
	}

	if err := client.HealthCheck(context.Background()); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	logger.Info("LVM client connected successfully")

	return client, nil
}

// Close closes the LVM client connection
func (c *LVMClient) Close() error {
	if c.conn != nil {
		logger.Info("Closing LVM client connection")
		return c.conn.Close()
	}
	return nil
}

// HealthCheck checks the health status of the LVM client
func (c *LVMClient) HealthCheck(ctx context.Context) error {
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

// GetThinPoolMetrics get thin pool metrics from lvm monitor server
func (c *LVMClient) GetThinPoolMetrics(ctx context.Context, options *ThinPoolQueryOptions) ([]*stat.ThinPoolMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &pb.GetThinPoolMetricsRequest{}
	if options != nil {
		req.ThinPoolName = options.ThinPoolName
		req.NodeName = options.NodeName
		req.VgName = options.VGName
	}

	resp, err := c.client.GetThinPoolMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get thin pool metrics: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}

	// convert to internal type
	metrics := make([]*stat.ThinPoolMetrics, len(resp.Metrics))
	for i, protoMetric := range resp.Metrics {
		metrics[i] = convertFromProto(protoMetric)
	}

	// test:
	for _, metric := range metrics {
		fmt.Printf("Client metric: %+v\n", *metric)
	}

	return metrics, nil
}

// GetContainerFsStats get container filesystem stats from lvm monitor server
func (c *LVMClient) GetContainerFsStats(ctx context.Context) (*stat.FsStats, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &pb.GetContainerFsStatsRequest{}

	resp, err := c.client.GetContainerFsStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get container fs stats: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}

	// convert protobuf to internal type
	stats := convertFsStatsFromProto(resp.Stats)

	return stats, nil
}

// convertFromProto converts the ThinPoolMetricsProto to the ThinPoolMetrics
func convertFromProto(protoMetric *pb.ThinPoolMetricsProto) *stat.ThinPoolMetrics {

	return &stat.ThinPoolMetrics{
		Timestamp:           time.Unix(protoMetric.Timestamp, 0),
		ThinPoolName:        protoMetric.ThinPoolName,
		NodeName:            protoMetric.NodeName,
		VGName:              protoMetric.VgName,
		UUID:                protoMetric.Uuid,
		HealthStatus:        int(protoMetric.HealthStatus),
		ActiveStatus:        protoMetric.ActiveStatus,
		PoolName:            protoMetric.PoolName,
		TotalSize:           protoMetric.TotalSize,
		UsedSize:            protoMetric.UsedSize,
		VGFreeSize:          protoMetric.VgFreeSize,
		MetadataSize:        protoMetric.MetadataSize,
		MetadataUsedSize:    protoMetric.MetadataUsedSize,
		MetadataFreeSize:    protoMetric.MetadataFreeSize,
		DataPercent:         protoMetric.DataPercent,
		MetadataUsedPercent: protoMetric.MetadataUsedPercent,
		SnapshotUsedPercent: protoMetric.SnapshotUsedPercent,
		SnapshotCount:       int(protoMetric.SnapshotCount),
		SnapshotSize:        uint64(protoMetric.SnapshotSize),
		ReadIOPS:            protoMetric.ReadIops,
		WriteIOPS:           protoMetric.WriteIops,
		ReadLatency:         protoMetric.ReadLatency,
		WriteLatency:        protoMetric.WriteLatency,
	}
}

// ThinPoolQueryOptions is the options for the ThinPoolQuery
type ThinPoolQueryOptions struct {
	ThinPoolName string
	NodeName     string
	VGName       string
}

// convertFsStatsFromProto converts FsStatsProto to FsStats
func convertFsStatsFromProto(protoStats *pb.FsStatsProto) *stat.FsStats {
	stats := &stat.FsStats{
		Time: metav1.Unix(protoStats.Timestamp, 0),
	}

	if protoStats.AvailableBytes > 0 {
		availableBytes := protoStats.AvailableBytes
		stats.AvailableBytes = &availableBytes
	}
	if protoStats.CapacityBytes > 0 {
		capacityBytes := protoStats.CapacityBytes
		stats.CapacityBytes = &capacityBytes
	}
	if protoStats.UsedBytes > 0 {
		usedBytes := protoStats.UsedBytes
		stats.UsedBytes = &usedBytes
	}
	if protoStats.InodesFree > 0 {
		inodesFree := protoStats.InodesFree
		stats.InodesFree = &inodesFree
	}
	if protoStats.Inodes > 0 {
		inodes := protoStats.Inodes
		stats.Inodes = &inodes
	}
	if protoStats.InodesUsed > 0 {
		inodesUsed := protoStats.InodesUsed
		stats.InodesUsed = &inodesUsed
	}

	return stats
}
