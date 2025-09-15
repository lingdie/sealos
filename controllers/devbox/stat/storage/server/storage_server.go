package storageserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/labring/sealos/controllers/devbox/stat"
	pb "github.com/labring/sealos/controllers/devbox/stat/proto"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

type StorageServer struct {
	pb.UnimplementedStorageServiceServer
	provider stat.NodeStatsProvider
}

func NewStorageServer(storageType string) *StorageServer {
	return &StorageServer{
		provider: stat.NewNodeStatsProvider(storageType),
	}
}

func (s *StorageServer) GetStorageStats(ctx context.Context, req *pb.GetStorageStatsRequest) (*pb.GetStorageStatsResponse, error) {
	stats, err := s.provider.GetStorageStats(ctx)
	if err != nil {
		return &pb.GetStorageStatsResponse{
			Error: err.Error(),
		}, err
	}

	// convert to protobuf format
	statsProto := &pb.StorageStatsProto{
		Timestamp:   stats.Timestamp.Unix(),
		NodeName:    stats.NodeName,
		StorageType: stats.StorageType,
		DataPercent: stats.DataPercent,
	}

	if stats.CapacityBytes != nil {
		statsProto.CapacityBytes = *stats.CapacityBytes
	}
	if stats.AvailableBytes != nil {
		statsProto.AvailableBytes = *stats.AvailableBytes
	}
	if stats.UsedBytes != nil {
		statsProto.UsedBytes = *stats.UsedBytes
	}
	if stats.InodesFree != nil {
		statsProto.InodesFree = *stats.InodesFree
	}
	if stats.Inodes != nil {
		statsProto.Inodes = *stats.Inodes
	}
	if stats.InodesUsed != nil {
		statsProto.InodesUsed = *stats.InodesUsed
	}

	return &pb.GetStorageStatsResponse{
		Stats: statsProto,
	}, nil
}

func (s *StorageServer) GetStorageMetadata(ctx context.Context, req *pb.GetStorageMetadataRequest) (*pb.GetStorageMetadataResponse, error) {
	metadata, err := s.provider.GetStorageMetadata(ctx)
	if err != nil {
		return &pb.GetStorageMetadataResponse{
			Error: err.Error(),
		}, err
	}

	// convert to protobuf format
	metadataProto := &pb.StorageMetadataProto{
		Timestamp:           metadata.Timestamp.Unix(),
		StorageType:         metadata.StorageType,
		NodeName:            metadata.NodeName,
		MetadataUsedPercent: metadata.MetadataUsedPercent,
	}

	if metadata.MetadataCapacityBytes != nil {
		metadataProto.MetadataCapacityBytes = *metadata.MetadataCapacityBytes
	}
	if metadata.MetadataAvailableBytes != nil {
		metadataProto.MetadataAvailableBytes = *metadata.MetadataAvailableBytes
	}
	if metadata.MetadataUsedBytes != nil {
		metadataProto.MetadataUsedBytes = *metadata.MetadataUsedBytes
	}

	return &pb.GetStorageMetadataResponse{
		Metadata: metadataProto,
	}, nil
}

func (s *StorageServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	_, err := s.provider.GetStorageStats(ctx)
	if err != nil {
		logger.Info("[HealthCheck] failed to get storage stats", "error", err)
		return &pb.HealthCheckResponse{
			Healthy: false,
			Message: fmt.Sprintf("Storage server is unhealthy, err: %v", err),
		}, err
	}
	return &pb.HealthCheckResponse{
		Healthy: true,
		Message: "Storage server is healthy",
	}, nil
}

// StartStorageServer starts the Storage gRPC server
func StartStorageServer(ctx context.Context, port int, storageType string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Error("[StartStorageServer] failed to listen", "error", err)
		return fmt.Errorf("failed to listen, err: %v", err)
	}

	// TODO: In production, use proper TLS credentials
	// For development, we use insecure connection
	// In production, uncomment the following lines and provide proper certificates:
	// creds := credentials.NewTLS(&tls.Config{
	//     MinVersion: tls.VersionTLS12,
	// })
	// server := grpc.NewServer(grpc.Creds(creds))

	// Development mode - insecure connection
	server := grpc.NewServer()

	// register server
	pb.RegisterStorageServiceServer(server, NewStorageServer(storageType))

	// register healthy server
	grpc_health_v1.RegisterHealthServer(server, NewHealthServer(storageType))

	// reflect
	reflection.Register(server)

	// start server
	go func() {
		logger.Info("[StartStorageServer] Starting Storage gRPC server on port", "port", port)
		if err := server.Serve(listener); err != nil {
			logger.Error("[StartStorageServer] failed to start server", "error", err)
		}
	}()

	// graceful shutdown
	<-ctx.Done()
	logger.Info("Storage gRPC server shutting down...")

	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Storage gRPC server stopped gracefully")
	case <-time.After(10 * time.Second):
		logger.Info("Storage gRPC server shutdown timeout, forcing stop")
		server.Stop()
	}

	return nil
}

type HealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	provider stat.NodeStatsProvider
}

func NewHealthServer(storageType string) *HealthServer {
	return &HealthServer{
		provider: stat.NewNodeStatsProvider(storageType),
	}
}

// Check check healthy status by checking the thin pool metrics
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	_, err := s.provider.GetStorageStats(ctx)
	if err != nil {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, err
	}
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}
