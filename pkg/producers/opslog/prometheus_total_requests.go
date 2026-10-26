// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"strings"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	// Detailed requests with all dimensions
	totalRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radosgw_total_requests",
			Help: "Total number of requests processed with full detail",
		},
		[]string{"pod", "user", "tenant", "bucket", "method", "http_status"},
	)

	// Aggregated request counters
	totalRequestsPerUserCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radosgw_total_requests_per_user",
			Help: "Total requests aggregated per user (all buckets combined)",
		},
		[]string{"pod", "user", "tenant", "method", "http_status"},
	)

	totalRequestsPerBucketCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radosgw_total_requests_per_bucket",
			Help: "Total requests aggregated per bucket (all users combined)",
		},
		[]string{"pod", "tenant", "bucket", "method", "http_status"},
	)

	totalRequestsPerTenantCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radosgw_total_requests_per_tenant",
			Help: "Total requests aggregated per tenant (all users and buckets combined)",
		},
		[]string{"pod", "tenant", "method", "http_status"},
	)
)

func registerTotalRequestsMetrics(metricsConfig *MetricsConfig) {
	// Register detailed requests counter if enabled
	if metricsConfig.TrackRequestsDetailed {
		prometheus.MustRegister(totalRequestsCounter)
	}

	// Conditional registrations for aggregated metrics
	if metricsConfig.TrackRequestsPerUser {
		prometheus.MustRegister(totalRequestsPerUserCounter)
	}

	if metricsConfig.TrackRequestsPerBucket {
		prometheus.MustRegister(totalRequestsPerBucketCounter)
	}

	if metricsConfig.TrackRequestsPerTenant {
		prometheus.MustRegister(totalRequestsPerTenantCounter)
	}
}

func publishRequestCounters(diffMetrics *Metrics, cfg OpsLogConfig) {
	metricsConfig := cfg.MetricsConfig

	// Publish detailed requests - dedicated storage
	if metricsConfig.TrackRequestsDetailed {
		diffMetrics.RequestsDetailed.Range(func(key, requestCount any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) != 4 {
				return true
			}

			user, bucket, method, httpStatus := parts[0], parts[1], parts[2], parts[3]
			userStr, tenantStr := extractUserAndTenant(user)
			rqCount := float64(requestCount.(*atomic.Uint64).Load())

			if rqCount > 0 {
				totalRequestsCounter.With(prometheus.Labels{
					"pod":         cfg.PodName,
					"user":        userStr,
					"tenant":      tenantStr,
					"bucket":      bucket,
					"method":      method,
					"http_status": httpStatus,
				}).Add(rqCount)
			}
			return true
		})
	}

	// Publish per-user requests - dedicated storage
	if metricsConfig.TrackRequestsPerUser {
		diffMetrics.RequestsByUser.Range(func(key, requestCount any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) != 4 {
				log.Warn().Msgf("Invalid key format in RequestsByUser: %v", key)
				return true
			}

			user, _, method, httpStatus := parts[0], parts[1], parts[2], parts[3]
			userStr, tenantStr := extractUserAndTenant(user)
			rqCount := float64(requestCount.(*atomic.Uint64).Load())

			if rqCount > 0 {
				totalRequestsPerUserCounter.With(prometheus.Labels{
					"pod":         cfg.PodName,
					"user":        userStr,
					"tenant":      tenantStr,
					"method":      method,
					"http_status": httpStatus,
				}).Add(rqCount)
			}
			return true
		})
	}

	// Publish per-bucket requests - dedicated storage
	if metricsConfig.TrackRequestsPerBucket {
		diffMetrics.RequestsByBucket.Range(func(key, requestCount any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) != 4 {
				log.Warn().Msgf("Invalid key format in RequestsByBucket: %v", key)
				return true
			}

			user, bucket, method, httpStatus := parts[0], parts[1], parts[2], parts[3]
			_, tenantStr := extractUserAndTenant(user)
			rqCount := float64(requestCount.(*atomic.Uint64).Load())

			if rqCount > 0 {
				totalRequestsPerBucketCounter.With(prometheus.Labels{
					"pod":         cfg.PodName,
					"tenant":      tenantStr,
					"bucket":      bucket,
					"method":      method,
					"http_status": httpStatus,
				}).Add(rqCount)
			}
			return true
		})
	}

	// Publish per-tenant requests - dedicated storage
	if metricsConfig.TrackRequestsPerTenant {
		diffMetrics.RequestsByTenant.Range(func(key, requestCount any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) != 3 {
				return true
			}

			tenant, method, httpStatus := parts[0], parts[1], parts[2]
			rqCount := float64(requestCount.(*atomic.Uint64).Load())

			if rqCount > 0 {
				totalRequestsPerTenantCounter.With(prometheus.Labels{
					"pod":         cfg.PodName,
					"tenant":      tenant,
					"method":      method,
					"http_status": httpStatus,
				}).Add(rqCount)
			}
			return true
		})
	}
}
