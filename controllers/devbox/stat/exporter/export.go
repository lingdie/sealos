package exporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labring/sealos/controllers/devbox/stat"
	"github.com/labring/sealos/controllers/pkg/utils/logger"
)

// MetricsExporter is the interface for exporting metrics to VictoriaMetrics
type MetricsExporter interface {
	// ExportStorageStats exports storage stats to VictoriaMetrics
	ExportStorageStats(ctx context.Context, stats *stat.StorageStats) error

	// ExportStorageMetadata exports storage metadata to VictoriaMetrics
	ExportStorageMetadata(ctx context.Context, metadata *stat.StorageMetadata) error
}

type ExportConfig struct {
	VictoriaMetricsURL string
}

func DefaultExportConfig() *ExportConfig {
	return &ExportConfig{
		VictoriaMetricsURL: stat.VMImportURL,
	}
}

// VMMetricsExporter VictoriaMetrics exporter
type VMMetricsExporter struct {
	client *http.Client
	config *ExportConfig
}

// NewVMMetricsExporter creates a new VictoriaMetrics exporter
func NewVMMetricsExporter(config *ExportConfig) *VMMetricsExporter {
	if config == nil {
		config = DefaultExportConfig()
	}
	return &VMMetricsExporter{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
	}
}

// ExportStorageStats exports storage stats to VictoriaMetrics
func (v *VMMetricsExporter) ExportStorageStats(ctx context.Context, stats *stat.StorageStats) error {
	if stats == nil {
		return fmt.Errorf("storage stats is nil")
	}

	// build Prometheus format metrics
	metrics := v.buildStorageStatsMetrics(stats)

	// send to VictoriaMetrics
	return v.sendToVictoriaMetrics(ctx, metrics)
}

// ExportStorageMetadata exports storage metadata to VictoriaMetrics
func (v *VMMetricsExporter) ExportStorageMetadata(ctx context.Context, metadata *stat.StorageMetadata) error {
	if metadata == nil {
		return fmt.Errorf("storage metadata is nil")
	}

	// build Prometheus format metrics
	metrics := v.buildStorageMetadataMetrics(metadata)

	// send to VictoriaMetrics
	return v.sendToVictoriaMetrics(ctx, metrics)
}

// buildStorageStatsMetrics builds storage stats metrics
func (v *VMMetricsExporter) buildStorageStatsMetrics(stats *stat.StorageStats) []string {
	var metrics []string
	timestamp := stats.Timestamp.Unix()

	// base capacity metrics
	if stats.CapacityBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageCapacityBytes,
			stats.NodeName, stats.StorageType, *stats.CapacityBytes, timestamp))
	}

	if stats.AvailableBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageAvailableBytes,
			stats.NodeName, stats.StorageType, *stats.AvailableBytes, timestamp))
	}

	if stats.UsedBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageUsedBytes,
			stats.NodeName, stats.StorageType, *stats.UsedBytes, timestamp))
	}

	// usage rate metrics
	metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %f %d", StorageUsagePercent,
		stats.NodeName, stats.StorageType, stats.DataPercent, timestamp))

	// inode metrics
	if stats.Inodes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageInodesTotal,
			stats.NodeName, stats.StorageType, *stats.Inodes, timestamp))
	}

	if stats.InodesUsed != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageInodesUsed,
			stats.NodeName, stats.StorageType, *stats.InodesUsed, timestamp))
	}

	if stats.InodesFree != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageInodesFree,
			stats.NodeName, stats.StorageType, *stats.InodesFree, timestamp))
	}

	// extended metrics
	for key, value := range stats.Metrics {
		metrics = append(metrics, fmt.Sprintf("storage_%s{node=\"%s\",storage_type=\"%s\"} %d %d",
			key, stats.NodeName, stats.StorageType, value, timestamp))
	}

	return metrics
}

// buildStorageMetadataMetrics builds storage metadata metrics
func (v *VMMetricsExporter) buildStorageMetadataMetrics(metadata *stat.StorageMetadata) []string {
	var metrics []string
	timestamp := metadata.Timestamp.Unix()

	// metadata capacity metrics
	if metadata.MetadataCapacityBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageMetadataCapacityBytes,
			metadata.NodeName, metadata.StorageType, *metadata.MetadataCapacityBytes, timestamp))
	}

	if metadata.MetadataAvailableBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageMetadataAvailableBytes,
			metadata.NodeName, metadata.StorageType, *metadata.MetadataAvailableBytes, timestamp))
	}

	if metadata.MetadataUsedBytes != nil {
		metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %d %d", StorageMetadataUsedBytes,
			metadata.NodeName, metadata.StorageType, *metadata.MetadataUsedBytes, timestamp))
	}

	// metadata usage rate
	metrics = append(metrics, fmt.Sprintf("%s{node=\"%s\",storage_type=\"%s\"} %f %d", StorageMetadataUsagePercent,
		metadata.NodeName, metadata.StorageType, metadata.MetadataUsedPercent, timestamp))

	// extended metrics
	for key, value := range metadata.Metrics {
		metrics = append(metrics, fmt.Sprintf("storage_metadata_%s{node=\"%s\",storage_type=\"%s\"} %s %d",
			key, metadata.NodeName, metadata.StorageType, value, timestamp))
	}

	return metrics
}

// sendToVictoriaMetrics sends data to VictoriaMetrics
func (v *VMMetricsExporter) sendToVictoriaMetrics(ctx context.Context, metrics []string) error {
	if len(metrics) == 0 {
		return fmt.Errorf("no metrics to send")
	}

	// merge all metrics
	data := strings.Join(metrics, "\n")

	logger.Info("[VMMetricsExporter] Sending %d metrics to VM", len(metrics))
	logger.Debug("[VMMetricsExporter] Metrics data:\n%s", data)

	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", v.config.VictoriaMetricsURL, strings.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	// send request
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	logger.Info("[VMMetricsExporter] VM response status: %d, body: %s", resp.StatusCode, string(body))

	// check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("VM returned error status: %d, body: %s", resp.StatusCode, string(body))
	}

	logger.Info("[VMMetricsExporter] Successfully sent data to VM (status: %d)", resp.StatusCode)
	return nil
}
