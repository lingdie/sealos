package lvmmonitorserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/labring/sealos/controllers/devbox/internal/stat"
	pb "github.com/labring/sealos/controllers/devbox/internal/stat/protoc"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

type LVMMonitorGRPCServer struct {
	pb.UnimplementedLVMMonitorServiceServer
	lvmMonitor *stat.NodeStatsProviderImpl
}

// NewLVMMonitorGRPCServer creates a new LVMMonitorGRPCServer
func NewLVMMonitorGRPCServer() *LVMMonitorGRPCServer {
	return &LVMMonitorGRPCServer{
		lvmMonitor: &stat.NodeStatsProviderImpl{},
	}
}

// GetThinPoolMetrics returns the thin pool metrics
func (s *LVMMonitorGRPCServer) GetThinPoolMetrics(ctx context.Context, req *pb.GetThinPoolMetricsRequest) (*pb.GetThinPoolMetricsResponse, error) {
	metrics, err := s.lvmMonitor.ThinPoolMetrics()
	if err != nil {
		logger.Info("[GetThinPoolMetrics] failed to get thin pool metrics", "error", err)
		return nil, err
	}
	protoMetrics := make([]*pb.ThinPoolMetricsProto, 0, len(metrics))
	for _, metric := range metrics {
		// filter metrics by thin pool name
		if req.ThinPoolName != "" && metric.ThinPoolName != req.ThinPoolName {
			continue
		}
		// filter metrics by node name
		if req.NodeName != "" && metric.NodeName != req.NodeName {
			continue
		}
		// filter metrics by vg name
		if req.VgName != "" && metric.VGName != req.VgName {
			continue
		}
		protoMetrics = append(protoMetrics, convertToProto(metric))
	}
	return &pb.GetThinPoolMetricsResponse{
		Metrics: protoMetrics,
	}, nil
}

// GetThinPoolStats returns the thin pool stats
func (s *LVMMonitorGRPCServer) GetThinPoolStats(ctx context.Context, req *pb.GetThinPoolStatsRequest) (*pb.GetThinPoolStatsResponse, error) {
	logger.Info("[GetThinPoolStats] not implemented")
	return nil, nil
}

// GetVolumeGroupInfo returns the volume group info
func (s *LVMMonitorGRPCServer) GetVolumeGroupInfo(ctx context.Context, req *pb.GetVolumeGroupInfoRequest) (*pb.GetVolumeGroupInfoResponse, error) {
	logger.Info("[GetVolumeGroupInfo] not implemented")
	return nil, nil
}

// HealthCheck checks the health status of the LVM Monitor
func (s *LVMMonitorGRPCServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	_, err := s.lvmMonitor.ThinPoolMetrics()
	if err != nil {
		logger.Info("[HealthCheck] failed to get thin pool metrics", "error", err)
		return &pb.HealthCheckResponse{
			Healthy: false,
			Message: fmt.Sprintf("LVM monitor is unhealthy, err: %v", err),
		}, err
	}
	return &pb.HealthCheckResponse{
		Healthy: true,
		Message: "LVM monitor is healthy",
	}, nil
}

// convertToProto convert the ThinPoolMetrics to the ThinPoolMetricsProto
func convertToProto(metric *stat.ThinPoolMetrics) *pb.ThinPoolMetricsProto {
	return &pb.ThinPoolMetricsProto{
		Timestamp:           metric.Timestamp.Unix(),
		ThinPoolName:        metric.ThinPoolName,
		NodeName:            metric.NodeName,
		VgName:              metric.VGName,
		Uuid:                metric.UUID,
		HealthStatus:        int32(metric.HealthStatus),
		ActiveStatus:        metric.ActiveStatus,
		PoolName:            metric.PoolName,
		TotalSize:           metric.TotalSize,
		UsedSize:            metric.UsedSize,
		VgFreeSize:          metric.VGFreeSize,
		MetadataSize:        metric.MetadataSize,
		MetadataUsedSize:    metric.MetadataUsedSize,
		MetadataFreeSize:    metric.MetadataFreeSize,
		DataPercent:         metric.DataPercent,
		MetadataUsedPercent: metric.MetadataUsedPercent,
		SnapshotUsedPercent: metric.SnapshotUsedPercent,
		SnapshotCount:       int32(metric.SnapshotCount),
		SnapshotSize:        int64(metric.SnapshotSize),
		ReadIops:            metric.ReadIOPS,
		WriteIops:           metric.WriteIOPS,
		ReadLatency:         metric.ReadLatency,
		WriteLatency:        metric.WriteLatency,
	}
}

// StartLVMMonitorServer starts the LVM Monitor gRPC server
func StartLVMMonitorServer(ctx context.Context, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Error("[StartLVMMonitorServer] failed to listen", "error", err)
		return fmt.Errorf("failed to listen, err: %v", err)
	}
	server := grpc.NewServer()

	// register server
	pb.RegisterLVMMonitorServiceServer(server, NewLVMMonitorGRPCServer())

	// register healthy server
	grpc_health_v1.RegisterHealthServer(server, NewHealthServer())

	// reflect
	reflection.Register(server)

	// start server
	go func() {
		logger.Info("[StartLVMMonitorServer] Starting LVM Monitor gRPC server on port %d", port)
		if err := server.Serve(listener); err != nil {
			logger.Error("[StartLVMMonitorServer] failed to start server", "error", err)
		}
	}()

	// graceful shutdown
	<-ctx.Done()
	logger.Info("gRPC server shutting down...")

	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("gRPC server stopped gracefully")
	case <-time.After(10 * time.Second):
		logger.Info("gRPC server shutdown timeout, forcing stop")
		server.Stop()
	}

	return nil
}

type HealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	lvmMonitor *stat.NodeStatsProviderImpl
}

func NewHealthServer() *HealthServer {
	return &HealthServer{
		lvmMonitor: &stat.NodeStatsProviderImpl{},
	}
}

// Check check healthy status by checking the thin pool metrics
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	_, err := s.lvmMonitor.ThinPoolMetrics()
	if err != nil {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, err
	}
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch continue to check healthy status
func (s *HealthServer) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(30 * time.Second):
			status := grpc_health_v1.HealthCheckResponse_SERVING

			// check lvm monitor health
			_, err := s.lvmMonitor.ThinPoolMetrics()
			if err != nil {
				status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
			}

			// send health status to client
			if err := stream.Send(&grpc_health_v1.HealthCheckResponse{
				Status: status,
			}); err != nil {
				return err
			}
		}
	}
}
