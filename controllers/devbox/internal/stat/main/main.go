package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labring/sealos/controllers/devbox/internal/stat"
	lvmmonitorserver "github.com/labring/sealos/controllers/devbox/internal/stat/lvm_monitor/lvm_monitor_server"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

// provide a main function to start the stat server and lvm exporter server
func main() {
	var (
		grpcPort         = flag.Int("grpc-port", stat.DefaultGRPCPort, "The port to listen on for the gRPC server")
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
	lvmMonitor := &stat.NodeStatsProviderImpl{}

	// GRPC server
	if *enableGRPC {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := lvmmonitorserver.StartLVMMonitorServer(ctx, *grpcPort); err != nil {
				logger.Error("failed to start LVM monitor server", "error", err)
				cancel()
			}
		}()
	}

	// Monitoring task
	if *enableMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startMonitoringTask(ctx, *monitorInterval, lvmMonitor); err != nil {
				logger.Error("failed to start monitoring task", "error", err)
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

	logger.Info("LVM Monitor stopped")
}

func startMonitoringTask(ctx context.Context, interval time.Duration, lvmMonitor *stat.NodeStatsProviderImpl) error {
	ticker := time.NewTicker(interval)
	logger.Info("starting monitoring task with interval %v", interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("monitoring task stopped")
			return nil
		case <-ticker.C:
			logger.Info("Collecting LVM metrics at time: %v...", time.Now())
			metrics, err := lvmMonitor.ThinPoolMetrics()
			if err != nil {
				logger.Error("failed to get thin pool metrics", "error", err)
				continue
			}
			if len(metrics) == 0 {
				logger.Info("no thin pool metrics found")
				continue
			}
			logger.Info("Exporting %d metrics to VictoriaMetrics...", len(metrics))
			if err := lvmMonitor.ExportToVictoriaMetrics(metrics); err != nil {
				logger.Error("failed to export metrics to VictoriaMetrics", "error", err)
			}
			logger.Info("metrics exported to VictoriaMetrics")
		}
	}
}
