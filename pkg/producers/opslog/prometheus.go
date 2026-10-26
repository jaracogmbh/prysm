// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	previousMetrics *Metrics = nil
)

// LatencyObs is the function that should be called during request processing
// to record latency observations. It's set up during initialization.
var LatencyObs func(user, tenant, bucket, method string, seconds float64)

// initPrometheusSettings initializes and registers all Prometheus metrics based on configuration
func initPrometheusSettings(cfg *OpsLogConfig) {
	metricsConfig := &cfg.MetricsConfig

	// Apply shortcuts and migrations
	metricsConfig.ApplyShortcuts()

	// Register total requests metrics
	registerTotalRequestsMetrics(metricsConfig)

	// Register method-based metrics
	registerMethodMetrics(metricsConfig)

	// Register operation-based metrics
	registerOperationMetrics(metricsConfig)

	// Register status code metrics
	registerStatusMetrics(metricsConfig)

	// Register bytes metrics
	registerBytesMetrics(metricsConfig)

	// Register error metrics
	registerErrorMetrics(metricsConfig)

	// Register IP-based metrics
	registerIPMetrics(metricsConfig)

	// Register latency metrics and set up LatencyObs function
	registerLatencyMetrics(metricsConfig)

	// Register dedicated low-cardinality SLI metrics for per-tenant SLOs
	if metricsConfig.TrackBucketSLO {
		sliCfg := buildSLICollectorConfig(metricsConfig, cfg.Region)
		registerSLIMetrics(sliCfg)
	}

	// Register audit drop counters
	registerAuditMetrics()

	// Set up the global LatencyObs function
	LatencyObs = latencyObs
}

// PublishToPrometheus updates Prometheus metrics from aggregated data
func PublishToPrometheus(totalMetrics *Metrics, cfg OpsLogConfig) {
	// Initialize previousMetrics as empty on first call
	if previousMetrics == nil {
		previousMetrics = NewMetrics() // Empty metrics
	}

	// Snapshot current total
	currentMetrics := totalMetrics.Clone()

	// Always compute diff (first time will be current - empty = current)
	diffMetrics := SubtractMetrics(currentMetrics, previousMetrics)
	// Update snapshot for next interval
	previousMetrics = currentMetrics

	// Publish the delta (which equals full state on first call)
	publishRequestCounters(diffMetrics, cfg)

	publishMethodMetrics(diffMetrics, cfg)

	publishOperationMetrics(diffMetrics, cfg)

	publishStatusMetrics(diffMetrics, cfg)

	publishBytesCounters(diffMetrics, cfg)

	publishErrorCounters(diffMetrics, cfg)

	publishIPGauges(currentMetrics, cfg)

	log.Info().Msg("Updated Prometheus metrics for users and buckets")
}

// StartPrometheusServer starts the HTTP server for Prometheus metrics endpoint
func StartPrometheusServer(port int, cfg *OpsLogConfig) {
	// Initialize Prometheus settings based on the configuration
	initPrometheusSettings(cfg)

	// Start the Prometheus HTTP server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info().Msgf("starting prometheus metrics server on :%d", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			log.Fatal().Err(err).Msg("error starting prometheus metrics server")
		}
	}()
}

// buildSLICollectorConfig constructs an SLICollectorConfig from MetricsConfig string fields.
func buildSLICollectorConfig(mc *MetricsConfig, region string) SLICollectorConfig {
	cfg := SLICollectorConfig{
		StaleTTL: 24 * time.Hour,
		Region:   region,
	}

	if mc.BucketSLOStaleTTL != "" {
		if d, err := time.ParseDuration(mc.BucketSLOStaleTTL); err == nil && d > 0 {
			cfg.StaleTTL = d
		} else {
			log.Warn().Str("value", mc.BucketSLOStaleTTL).Msg("invalid or non-positive bucket_slo_stale_ttl, using default 24h")
		}
	}
	if mc.BucketSLOReapInterval != "" {
		if d, err := time.ParseDuration(mc.BucketSLOReapInterval); err == nil && d > 0 {
			cfg.ReapInterval = d
		} else {
			log.Warn().Str("value", mc.BucketSLOReapInterval).Msg("invalid or non-positive bucket_slo_reap_interval, using default stale_ttl/4")
		}
	}

	return cfg
}
