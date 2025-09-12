package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labring/sealos/controllers/devbox/stat"
	"github.com/labring/sealos/controllers/devbox/stat/exporter"
	storageserver "github.com/labring/sealos/controllers/devbox/stat/storage/server"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

// provide a main function to start the stat server and exporter server
func main() {
	var (
		grpcPort         = flag.Int("grpc-port", stat.DefaultGRPCPort, "The port to listen on for the gRPC server")
		storageType      = flag.String("storage-type", stat.StorageTypeLVM, "The type of storage to monitor")
		monitorInterval  = flag.Duration("monitor-interval", stat.DefaultMonitorInterval, "The interval to monitor the LVM metrics")
		enableGRPC       = flag.Bool("enable-grpc", true, "Enable LVM gRPC server")
		enableMonitoring = flag.Bool("enable-monitoring", true, "Enable monitoring")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	storageProvider := stat.NewNodeStatsProvider(*storageType)

	exportConfig := exporter.DefaultExportConfig()
	exporter := exporter.NewVMMetricsExporter(exportConfig)

	// GRPC server
	if *enableGRPC {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := storageserver.StartStorageServer(ctx, *grpcPort, *storageType); err != nil {
				logger.Error("failed to start storage server", "error", err)
				cancel()
			}
		}()
	}

	// Storage monitoring task
	if *enableMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startMonitoringTask(ctx, *monitorInterval, storageProvider, exporter); err != nil {
				logger.Error("failed to start storage monitoring task", "error", err)
				cancel()
			}
		}()
	}

	select {
	case sig := <-sigChan:
		logger.Info("received signal %s, starting to graceful shut down", sig)
	case <-ctx.Done():
		logger.Info("context done, starting to graceful shut down")
	}

	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All goroutines finished")
	case <-time.After(30 * time.Second):
		logger.Warn("Timeout waiting for goroutines to finish")
	}

	logger.Info("Storage Monitor stopped")
}

func startMonitoringTask(ctx context.Context, interval time.Duration, storageProvider stat.NodeStatsProvider, exporter exporter.MetricsExporter) error {
	ticker := time.NewTicker(interval)
	logger.Info("starting storage monitoring task with interval %v", interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("storage monitoring task stopped")
			return nil
		case <-ticker.C:
			logger.Info("Collecting storage metrics at time: %v...", time.Now())

			// collect storage stats
			stats, err := storageProvider.GetStorageStats(ctx)
			if err != nil {
				logger.Error("failed to get storage stats", "error", err)
				continue
			}

			// collect storage metadata
			metadata, err := storageProvider.GetStorageMetadata(ctx)
			if err != nil {
				logger.Error("failed to get storage metadata", "error", err)
				continue
			}

			// export storage stats
			logger.Info("Exporting storage stats to VictoriaMetrics...")
			if err := exporter.ExportStorageStats(ctx, stats); err != nil {
				logger.Error("failed to export storage stats to VictoriaMetrics", "error", err)
			} else {
				logger.Info("storage stats exported to VictoriaMetrics successfully")
			}

			// export storage metadata
			logger.Info("Exporting storage metadata to VictoriaMetrics...")
			if err := exporter.ExportStorageMetadata(ctx, metadata); err != nil {
				logger.Error("failed to export storage metadata to VictoriaMetrics", "error", err)
			} else {
				logger.Info("storage metadata exported to VictoriaMetrics successfully")
			}
		}
	}
}
