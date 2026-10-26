// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
)

type Metrics struct {
	TotalRequests atomic.Uint64
	BytesSent     atomic.Uint64
	BytesReceived atomic.Uint64
	Errors        atomic.Uint64

	// Request tracking - each serves specific Prometheus metrics
	RequestsDetailed sync.Map // "user|bucket|method|http_status" -> *atomic.Uint64
	RequestsByUser   sync.Map // "user|bucket|method|http_status" -> *atomic.Uint64 (duplicate but for clarity)
	RequestsByBucket sync.Map // "user|bucket|method|http_status" -> *atomic.Uint64
	RequestsByTenant sync.Map // "tenant|method|http_status" -> *atomic.Uint64

	// Method-based tracking - dedicated maps for each aggregation level
	RequestsByMethodDetailed  sync.Map // "user|bucket|method" -> *atomic.Uint64
	RequestsByMethodPerUser   sync.Map // "user|method" -> *atomic.Uint64
	RequestsByMethodPerBucket sync.Map // "tenant|bucket|method" -> *atomic.Uint64
	RequestsByMethodPerTenant sync.Map // "tenant|method" -> *atomic.Uint64
	RequestsByMethodGlobal    sync.Map // "method" -> *atomic.Uint64

	// Operation-based tracking - dedicated maps for each aggregation level
	RequestsByOperationDetailed  sync.Map // "user|bucket|operation|method" -> *atomic.Uint64
	RequestsByOperationPerUser   sync.Map // "user|operation|method" -> *atomic.Uint64
	RequestsByOperationPerBucket sync.Map // "tenant|bucket|operation|method" -> *atomic.Uint64
	RequestsByOperationPerTenant sync.Map // "tenant|operation|method" -> *atomic.Uint64
	RequestsByOperationGlobal    sync.Map // "operation|method" -> *atomic.Uint64

	// Status-based tracking - dedicated maps for each aggregation level
	RequestsByStatusDetailed  sync.Map // "user|bucket|status" -> *atomic.Uint64
	RequestsByStatusPerUser   sync.Map // "user|status" -> *atomic.Uint64
	RequestsByStatusPerBucket sync.Map // "tenant|bucket|status" -> *atomic.Uint64
	RequestsByStatusPerTenant sync.Map // "tenant|status" -> *atomic.Uint64

	// Keep the simple status tracking for basic status code metrics
	RequestsPerStatusCode sync.Map // "http_status" -> *atomic.Uint64

	// LatencyObs records a single request‐latency observation into the
	// `requestsDurationHistogram`, which is registered once at startup.
	// Because the histogram lives for the entire process life and is never
	// re‐initialized or cleared, its buckets continuously accumulate across
	// scrape intervals—ensuring a true cumulative histogram that Prometheus
	// can derive accurate rates and quantiles from.
	LatencyObs func(user string, tenant string, bucket string, method string, seconds float64)

	// Bytes tracking - dedicated maps for each aggregation level
	// Sent bytes
	BytesSentDetailed  sync.Map // "user|bucket" -> *atomic.Uint64
	BytesSentPerUser   sync.Map // "user" -> *atomic.Uint64
	BytesSentPerBucket sync.Map // "tenant|bucket" -> *atomic.Uint64
	BytesSentPerTenant sync.Map // "tenant" -> *atomic.Uint64

	// Received bytes
	BytesReceivedDetailed  sync.Map // "user|bucket" -> *atomic.Uint64
	BytesReceivedPerUser   sync.Map // "user" -> *atomic.Uint64
	BytesReceivedPerBucket sync.Map // "tenant|bucket" -> *atomic.Uint64
	BytesReceivedPerTenant sync.Map // "tenant" -> *atomic.Uint64

	// Error tracking - dedicated maps for each aggregation level
	ErrorsDetailed  sync.Map // "user|bucket|http_status" -> *atomic.Uint64
	ErrorsPerUser   sync.Map // "user|http_status" -> *atomic.Uint64
	ErrorsPerBucket sync.Map // "tenant|bucket|http_status" -> *atomic.Uint64
	ErrorsPerTenant sync.Map // "tenant|http_status" -> *atomic.Uint64
	ErrorsPerStatus sync.Map // "http_status" -> *atomic.Uint64
	ErrorsPerIP     sync.Map // "ip|tenant|http_status" -> *atomic.Uint64

	// Enhanced error tracking for timeout and connection issues
	TimeoutErrors    sync.Map // "user|bucket|timeout_type" -> *atomic.Uint64
	ErrorsByCategory sync.Map // "tenant|bucket|category|status" -> *atomic.Uint64

	// IP-based tracking - dedicated maps for each aggregation level
	// Request tracking by IP
	RequestsByIPDetailed           sync.Map // "user|ip" -> *atomic.Uint64
	RequestsPerIPPerTenant         sync.Map // "tenant|ip" -> *atomic.Uint64
	RequestsPerTenantFromIP        sync.Map // "tenant" -> *atomic.Uint64
	RequestsByIPBucketMethodTenant sync.Map // "ip|bucket|method|tenant" -> *atomic.Uint64

	// Bytes sent tracking by IP
	BytesSentByIPDetailed    sync.Map // "user|ip" -> *atomic.Uint64
	BytesSentPerIPPerTenant  sync.Map // "tenant|ip" -> *atomic.Uint64
	BytesSentPerTenantFromIP sync.Map // "tenant" -> *atomic.Uint64

	// Bytes received tracking by IP
	BytesReceivedByIPDetailed    sync.Map // "user|ip" -> *atomic.Uint64
	BytesReceivedPerIPPerTenant  sync.Map // "tenant|ip" -> *atomic.Uint64
	BytesReceivedPerTenantFromIP sync.Map // "tenant" -> *atomic.Uint64
}

func NewMetrics(obs ...func(user string, tenant string, bucket string, method string, seconds float64)) *Metrics {
	var cb func(user, tenant, bucket, method string, seconds float64)
	if len(obs) > 0 {
		cb = obs[0]
	} else {
		// default no‐op so nobody ever has a nil-pointer
		cb = func(_, _, _, _ string, _ float64) {}
	}
	return &Metrics{
		LatencyObs: cb,
	}
}

// Convert metrics to a JSON-friendly struct
func (m *Metrics) ToJSON(metricsConfig *MetricsConfig) ([]byte, error) {
	data := map[string]any{
		"total_requests": m.TotalRequests.Load(),
		"bytes_sent":     m.BytesSent.Load(),
		"bytes_received": m.BytesReceived.Load(),
		"errors":         m.Errors.Load(),
	}

	if metricsConfig.TrackRequestsDetailed {
		data["requests_detailed"] = loadSyncMap(&m.RequestsDetailed)
	}

	if metricsConfig.TrackRequestsPerUser {
		data["requests_by_user"] = loadSyncMap(&m.RequestsByUser)
	}

	if metricsConfig.TrackRequestsPerBucket {
		data["requests_by_bucket"] = loadSyncMap(&m.RequestsByBucket)
	}

	if metricsConfig.TrackRequestsPerTenant {
		data["requests_by_tenant"] = loadSyncMap(&m.RequestsByTenant)
	}

	if metricsConfig.TrackRequestsByMethodDetailed {
		data["requests_by_method_detailed"] = loadSyncMap(&m.RequestsByMethodDetailed)
	}

	if metricsConfig.TrackRequestsByMethodPerUser {
		data["requests_by_method_per_user"] = loadSyncMap(&m.RequestsByMethodPerUser)
	}

	if metricsConfig.TrackRequestsByMethodPerBucket {
		data["requests_by_method_per_bucket"] = loadSyncMap(&m.RequestsByMethodPerBucket)
	}

	if metricsConfig.TrackRequestsByMethodPerTenant {
		data["requests_by_method_per_tenant"] = loadSyncMap(&m.RequestsByMethodPerTenant)
	}

	if metricsConfig.TrackRequestsByMethodGlobal {
		data["requests_by_method_global"] = loadSyncMap(&m.RequestsByMethodGlobal)
	}

	if metricsConfig.TrackRequestsByOperationDetailed {
		data["requests_by_operation_detailed"] = loadSyncMap(&m.RequestsByOperationDetailed)
	}

	if metricsConfig.TrackRequestsByOperationPerUser {
		data["requests_by_operation_per_user"] = loadSyncMap(&m.RequestsByOperationPerUser)
	}

	if metricsConfig.TrackRequestsByOperationPerBucket {
		data["requests_by_operation_per_bucket"] = loadSyncMap(&m.RequestsByOperationPerBucket)
	}

	if metricsConfig.TrackRequestsByOperationPerTenant {
		data["requests_by_operation_per_tenant"] = loadSyncMap(&m.RequestsByOperationPerTenant)
	}

	if metricsConfig.TrackRequestsByOperationGlobal {
		data["requests_by_operation_global"] = loadSyncMap(&m.RequestsByOperationGlobal)
	}

	if metricsConfig.TrackRequestsByStatusDetailed {
		data["requests_by_status_detailed"] = loadSyncMap(&m.RequestsByStatusDetailed)
	}

	if metricsConfig.TrackRequestsByStatusPerUser {
		data["requests_by_status_per_user"] = loadSyncMap(&m.RequestsByStatusPerUser)
	}
	if metricsConfig.TrackRequestsByStatusPerBucket {
		data["requests_by_status_per_bucket"] = loadSyncMap(&m.RequestsByStatusPerBucket)
	}

	if metricsConfig.TrackRequestsByStatusPerTenant {
		data["requests_by_status_per_tenant"] = loadSyncMap(&m.RequestsByStatusPerTenant)
	}

	if metricsConfig.TrackRequestsByStatusDetailed {
		data["requests_per_status"] = loadSyncMap(&m.RequestsPerStatusCode)
	}

	if metricsConfig.TrackRequestsByIPDetailed {
		data["requests_by_ip"] = loadSyncMap(&m.RequestsByIPDetailed)
	}
	if metricsConfig.TrackRequestsByIPBucketMethodTenant {
		data["requests_by_ip_bucket_method_tenant"] = loadSyncMap(&m.RequestsByIPBucketMethodTenant)
	}

	// Conditional fields (bytes tracking)
	if metricsConfig.TrackBytesSentDetailed {
		data["bytes_sent_detailed"] = loadSyncMap(&m.BytesSentDetailed)
	}

	if metricsConfig.TrackBytesSentPerUser {
		data["bytes_sent_per_user"] = loadSyncMap(&m.BytesSentPerUser)
	}

	if metricsConfig.TrackBytesSentPerBucket {
		data["bytes_sent_per_bucket"] = loadSyncMap(&m.BytesSentPerBucket)
	}
	if metricsConfig.TrackBytesSentPerTenant {
		data["bytes_sent_per_tenant"] = loadSyncMap(&m.BytesSentPerTenant)
	}

	if metricsConfig.TrackBytesReceivedDetailed {
		data["bytes_received_detailed"] = loadSyncMap(&m.BytesReceivedDetailed)
	}

	if metricsConfig.TrackBytesReceivedPerUser {
		data["bytes_received_per_user"] = loadSyncMap(&m.BytesReceivedPerUser)
	}

	if metricsConfig.TrackBytesReceivedPerBucket {
		data["bytes_received_per_bucket"] = loadSyncMap(&m.BytesReceivedPerBucket)
	}
	if metricsConfig.TrackBytesReceivedPerTenant {
		data["bytes_received_per_tenant"] = loadSyncMap(&m.BytesReceivedPerTenant)
	}

	if metricsConfig.TrackBytesSentByIPDetailed {
		data["bytes_sent_by_ip"] = loadSyncMap(&m.BytesSentByIPDetailed)
	}

	if metricsConfig.TrackBytesReceivedByIPDetailed {
		data["bytes_received_by_ip"] = loadSyncMap(&m.BytesReceivedByIPDetailed)
	}

	// Conditional fields (errors tracking)
	if metricsConfig.TrackErrorsDetailed {
		data["errors_detailed"] = loadSyncMap(&m.ErrorsDetailed)
	}
	if metricsConfig.TrackErrorsPerUser {
		data["errors_per_user"] = loadSyncMap(&m.ErrorsPerUser)
	}

	if metricsConfig.TrackErrorsPerBucket {
		data["errors_per_bucket"] = loadSyncMap(&m.ErrorsPerBucket)
	}

	if metricsConfig.TrackErrorsPerTenant {
		data["errors_per_tenant"] = loadSyncMap(&m.ErrorsPerTenant)
	}

	if metricsConfig.TrackErrorsPerStatus {
		data["errors_per_status"] = loadSyncMap(&m.ErrorsPerStatus)
	}
	if metricsConfig.TrackErrorsByIP {
		data["errors_per_ip"] = loadSyncMap(&m.ErrorsPerIP)
	}

	if metricsConfig.TrackTimeoutErrors {
		data["timeout_errors"] = loadSyncMap(&m.TimeoutErrors)
	}

	if metricsConfig.TrackErrorsByCategory {
		data["errors_by_category"] = loadSyncMap(&m.ErrorsByCategory)
	}

	return json.Marshal(data)
}

// Update increments metrics based on a new log entry
func (m *Metrics) Update(logEntry S3OperationLog, metricsConfig *MetricsConfig) {
	m.TotalRequests.Add(1)
	m.BytesSent.Add(uint64(logEntry.BytesSent))
	m.BytesReceived.Add(uint64(logEntry.BytesReceived))

	method := ExtractHTTPMethod(logEntry.URI)
	userStr, tenantStr := extractUserAndTenant(logEntry.User)

	if metricsConfig.TrackBucketSLO {
		// observeSLI writes directly to Prometheus via the custom collector rather than
		// accumulating in a sync.Map. The SLI metrics (radosgw_request_total and
		// radosgw_request_duration_seconds) are keyed by tenant, protocol, operation,
		// and status_class — aggregated at tenant level.
		observeSLI(logEntry, tenantStr)
	}

	if metricsConfig.TrackRequestsDetailed {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + method + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsDetailed, key)
	}
	if metricsConfig.TrackRequestsPerUser {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + method + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByUser, key)
	}
	if metricsConfig.TrackRequestsPerBucket {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + method + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByBucket, key)
	}
	if metricsConfig.TrackRequestsPerTenant {
		key := tenantStr + "|" + method + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByTenant, key)
	}

	if metricsConfig.TrackRequestsByMethodDetailed {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + method
		incrementSyncMap(&m.RequestsByMethodDetailed, key)
	}
	if metricsConfig.TrackRequestsByMethodPerUser {
		key := userStr + "|" + method
		incrementSyncMap(&m.RequestsByMethodPerUser, key)
	}

	if metricsConfig.TrackRequestsByMethodPerBucket {
		key := tenantStr + "|" + logEntry.Bucket + "|" + method
		incrementSyncMap(&m.RequestsByMethodPerBucket, key)
	}

	if metricsConfig.TrackRequestsByMethodPerTenant {
		key := tenantStr + "|" + method
		incrementSyncMap(&m.RequestsByMethodPerTenant, key)
	}

	if metricsConfig.TrackRequestsByMethodGlobal {
		key := method
		incrementSyncMap(&m.RequestsByMethodGlobal, key)
	}

	if metricsConfig.TrackRequestsByOperationDetailed {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + logEntry.Operation + "|" + method
		incrementSyncMap(&m.RequestsByOperationDetailed, key)
	}

	if metricsConfig.TrackRequestsByOperationPerUser {
		key := userStr + "|" + logEntry.Operation + "|" + method
		incrementSyncMap(&m.RequestsByOperationPerUser, key)
	}

	if metricsConfig.TrackRequestsByOperationPerBucket {
		key := tenantStr + "|" + logEntry.Bucket + "|" + logEntry.Operation + "|" + method
		incrementSyncMap(&m.RequestsByOperationPerBucket, key)
	}

	if metricsConfig.TrackRequestsByOperationPerTenant {
		key := tenantStr + "|" + logEntry.Operation + "|" + method
		incrementSyncMap(&m.RequestsByOperationPerTenant, key)
	}

	if metricsConfig.TrackRequestsByOperationGlobal {
		key := logEntry.Operation + "|" + method
		incrementSyncMap(&m.RequestsByOperationGlobal, key)
	}

	if metricsConfig.TrackRequestsByStatusDetailed {
		key := logEntry.User + "|" + logEntry.Bucket + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByStatusDetailed, key)
	}

	if metricsConfig.TrackRequestsByStatusPerUser {
		key := userStr + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByStatusPerUser, key)
	}

	if metricsConfig.TrackRequestsByStatusPerBucket {
		key := tenantStr + "|" + logEntry.Bucket + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByStatusPerBucket, key)
	}

	if metricsConfig.TrackRequestsByStatusPerTenant {
		key := tenantStr + "|" + logEntry.HTTPStatus
		incrementSyncMap(&m.RequestsByStatusPerTenant, key)
	}

	incrementSyncMap(&m.RequestsPerStatusCode, logEntry.HTTPStatus)

	if metricsConfig.TrackRequestsByIPDetailed {
		key := logEntry.User + "|" + logEntry.RemoteAddr
		incrementSyncMap(&m.RequestsByIPDetailed, key)
	}

	if metricsConfig.TrackRequestsByIPPerTenant {
		key := tenantStr + "|" + logEntry.RemoteAddr
		incrementSyncMap(&m.RequestsPerIPPerTenant, key)
	}

	if metricsConfig.TrackRequestsByIPGlobalPerTenant {
		incrementSyncMap(&m.RequestsPerTenantFromIP, tenantStr)
	}

	if metricsConfig.TrackRequestsByIPBucketMethodTenant {
		key := logEntry.RemoteAddr + "|" + logEntry.Bucket + "|" + method + "|" + tenantStr
		incrementSyncMap(&m.RequestsByIPBucketMethodTenant, key)
	}

	// Bytes Sent Tracking - store in dedicated maps based on enabled metrics
	if logEntry.BytesSent > 0 {
		if metricsConfig.TrackBytesSentDetailed {
			key := logEntry.User + "|" + logEntry.Bucket
			incrementSyncMapValue(&m.BytesSentDetailed, key, uint64(logEntry.BytesSent))
		}

		if metricsConfig.TrackBytesSentPerUser {
			incrementSyncMapValue(&m.BytesSentPerUser, userStr, uint64(logEntry.BytesSent))
		}

		if metricsConfig.TrackBytesSentPerBucket {
			key := tenantStr + "|" + logEntry.Bucket
			incrementSyncMapValue(&m.BytesSentPerBucket, key, uint64(logEntry.BytesSent))
		}

		if metricsConfig.TrackBytesSentPerTenant {
			incrementSyncMapValue(&m.BytesSentPerTenant, tenantStr, uint64(logEntry.BytesSent))
		}
		if metricsConfig.TrackBytesSentByIPDetailed {
			key := logEntry.User + "|" + logEntry.RemoteAddr
			incrementSyncMapValue(&m.BytesSentByIPDetailed, key, uint64(logEntry.BytesSent))
		}

		if metricsConfig.TrackBytesSentByIPPerTenant {
			key := tenantStr + "|" + logEntry.RemoteAddr
			incrementSyncMapValue(&m.BytesSentPerIPPerTenant, key, uint64(logEntry.BytesSent))
		}

		if metricsConfig.TrackBytesSentByIPGlobalPerTenant {
			incrementSyncMapValue(&m.BytesSentPerTenantFromIP, tenantStr, uint64(logEntry.BytesSent))
		}
	}

	// Bytes Received Tracking - store in dedicated maps based on enabled metrics
	if logEntry.BytesReceived > 0 {
		if metricsConfig.TrackBytesReceivedDetailed {
			key := logEntry.User + "|" + logEntry.Bucket
			incrementSyncMapValue(&m.BytesReceivedDetailed, key, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedPerUser {
			incrementSyncMapValue(&m.BytesReceivedPerUser, userStr, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedPerBucket {
			key := tenantStr + "|" + logEntry.Bucket
			incrementSyncMapValue(&m.BytesReceivedPerBucket, key, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedPerTenant {
			incrementSyncMapValue(&m.BytesReceivedPerTenant, tenantStr, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedByIPDetailed {
			key := logEntry.User + "|" + logEntry.RemoteAddr
			incrementSyncMapValue(&m.BytesReceivedByIPDetailed, key, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedByIPPerTenant {
			key := tenantStr + "|" + logEntry.RemoteAddr
			incrementSyncMapValue(&m.BytesReceivedPerIPPerTenant, key, uint64(logEntry.BytesReceived))
		}

		if metricsConfig.TrackBytesReceivedByIPGlobalPerTenant {
			incrementSyncMapValue(&m.BytesReceivedPerTenantFromIP, tenantStr, uint64(logEntry.BytesReceived))
		}
	}

	if logEntry.HTTPStatus[0] != '2' {
		// Track timeout errors specifically
		if metricsConfig.TrackTimeoutErrors && IsTimeoutError(logEntry.HTTPStatus) {
			timeoutType := GetTimeoutType(logEntry.HTTPStatus)
			key := logEntry.User + "|" + logEntry.Bucket + "|" + timeoutType
			incrementSyncMap(&m.TimeoutErrors, key)
		}

		// Track errors by category
		if metricsConfig.TrackErrorsByCategory {
			errorCategory := CategorizeHTTPError(logEntry.HTTPStatus)
			key := tenantStr + "|" + logEntry.Bucket + "|" + errorCategory + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsByCategory, key)
		}

		// Existing error tracking
		if metricsConfig.TrackErrorsDetailed {
			key := logEntry.User + "|" + logEntry.Bucket + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsDetailed, key)
		}
		if metricsConfig.TrackErrorsPerUser {
			key := userStr + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsPerUser, key)
		}
		if metricsConfig.TrackErrorsPerBucket {
			key := tenantStr + "|" + logEntry.Bucket + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsPerBucket, key)
		}
		if metricsConfig.TrackErrorsPerTenant {
			key := tenantStr + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsPerTenant, key)
		}
		if metricsConfig.TrackErrorsPerStatus {
			incrementSyncMap(&m.ErrorsPerStatus, logEntry.HTTPStatus)
		}

		if metricsConfig.TrackErrorsByIP {
			key := logEntry.RemoteAddr + "|" + tenantStr + "|" + logEntry.HTTPStatus
			incrementSyncMap(&m.ErrorsPerIP, key)
		}

		m.Errors.Add(1)
	}

	// Latency Tracking
	if logEntry.TotalTime > 0 {
		if metricsConfig.TrackLatencyDetailed ||
			metricsConfig.TrackLatencyPerMethod ||
			metricsConfig.TrackLatencyPerBucket ||
			metricsConfig.TrackLatencyPerTenant ||
			metricsConfig.TrackLatencyPerUser ||
			metricsConfig.TrackLatencyPerBucketAndMethod {

			latencySec := float64(logEntry.TotalTime) / 1000.0
			userStr, tenantStr := extractUserAndTenant(logEntry.User)
			m.LatencyObs(userStr, tenantStr, logEntry.Bucket, method, latencySec)
		}
	}
}

// Reset function
func (m *Metrics) Reset() {
	m.TotalRequests.Store(0)
	m.BytesSent.Store(0)
	m.BytesReceived.Store(0)
	m.Errors.Store(0)

	resetSyncMap(&m.RequestsDetailed)
	resetSyncMap(&m.RequestsByUser)
	resetSyncMap(&m.RequestsByBucket)
	resetSyncMap(&m.RequestsByTenant)
	resetSyncMap(&m.RequestsByMethodDetailed)
	resetSyncMap(&m.RequestsByMethodPerUser)
	resetSyncMap(&m.RequestsByMethodPerBucket)
	resetSyncMap(&m.RequestsByMethodPerTenant)
	resetSyncMap(&m.RequestsByMethodGlobal)
	resetSyncMap(&m.RequestsByOperationDetailed)
	resetSyncMap(&m.RequestsByOperationPerUser)
	resetSyncMap(&m.RequestsByOperationPerBucket)
	resetSyncMap(&m.RequestsByOperationPerTenant)
	resetSyncMap(&m.RequestsByOperationGlobal)
	resetSyncMap(&m.RequestsByStatusDetailed)
	resetSyncMap(&m.RequestsByStatusPerUser)
	resetSyncMap(&m.RequestsByStatusPerBucket)
	resetSyncMap(&m.RequestsByStatusPerTenant)
	resetSyncMap(&m.RequestsPerStatusCode)
	resetSyncMap(&m.BytesSentDetailed)
	resetSyncMap(&m.BytesSentPerUser)
	resetSyncMap(&m.BytesSentPerBucket)
	resetSyncMap(&m.BytesSentPerTenant)
	resetSyncMap(&m.BytesReceivedDetailed)
	resetSyncMap(&m.BytesReceivedPerUser)
	resetSyncMap(&m.BytesReceivedPerBucket)
	resetSyncMap(&m.BytesReceivedPerTenant)
	resetSyncMap(&m.ErrorsDetailed)
	resetSyncMap(&m.ErrorsPerUser)
	resetSyncMap(&m.ErrorsPerBucket)
	resetSyncMap(&m.ErrorsPerTenant)
	resetSyncMap(&m.ErrorsPerStatus)
	resetSyncMap(&m.ErrorsPerIP)
	resetSyncMap(&m.TimeoutErrors)
	resetSyncMap(&m.ErrorsByCategory)
	resetSyncMap(&m.RequestsByIPDetailed)
	resetSyncMap(&m.RequestsPerIPPerTenant)
	resetSyncMap(&m.RequestsPerTenantFromIP)
	resetSyncMap(&m.RequestsByIPBucketMethodTenant)
	resetSyncMap(&m.BytesSentByIPDetailed)
	resetSyncMap(&m.BytesSentPerIPPerTenant)
	resetSyncMap(&m.BytesSentPerTenantFromIP)
	resetSyncMap(&m.BytesReceivedByIPDetailed)
	resetSyncMap(&m.BytesReceivedPerIPPerTenant)
	resetSyncMap(&m.BytesReceivedPerTenantFromIP)
}

// Helper function: Update max atomic value
func updateMaxAtomic(target *atomic.Uint64, value uint64) {
	for {
		curr := target.Load()
		if value > curr {
			target.Store(value)
		} else {
			break
		}
	}
}

// Helper function: Update max value in sync.Map
func updateMaxSyncMap(m *sync.Map, key string, value uint64) {
	val, _ := m.LoadOrStore(key, new(atomic.Uint64))
	atomicVal := val.(*atomic.Uint64)
	updateMaxAtomic(atomicVal, value)
}

// Helper function: Update min atomic value
func updateMinAtomic(target *atomic.Uint64, value uint64) {
	for {
		curr := target.Load()
		if curr == 0 || value < curr {
			target.Store(value)
		} else {
			break
		}
	}
}

// Helper function: Update min value in sync.Map
func updateMinSyncMap(m *sync.Map, key string, value uint64) {
	val, _ := m.LoadOrStore(key, new(atomic.Uint64))
	atomicVal := val.(*atomic.Uint64)
	updateMinAtomic(atomicVal, value)
}

// Helper function: Increment sync.Map atomic value
func incrementSyncMap(m *sync.Map, key string) {
	if val, ok := m.Load(key); ok {
		val.(*atomic.Uint64).Add(1)
		return
	}

	newVal := new(atomic.Uint64)
	newVal.Add(1)
	actual, _ := m.LoadOrStore(key, newVal)
	if actual != newVal {
		actual.(*atomic.Uint64).Add(1)
	}
}

// Helper function: Increment sync.Map numeric values
func incrementSyncMapValue(m *sync.Map, key string, value uint64) {
	if val, ok := m.Load(key); ok {
		val.(*atomic.Uint64).Add(value)
		return
	}

	newVal := new(atomic.Uint64)
	newVal.Add(value)
	actual, _ := m.LoadOrStore(key, newVal)
	if actual != newVal {
		actual.(*atomic.Uint64).Add(value)
	}
}

// Helper function: Reset sync.Map
func resetSyncMap(m *sync.Map) {
	m.Range(func(key, _ any) bool {
		m.Delete(key)
		return true
	})
}

// Helper function: Convert sync.Map to map[string]uint64
func loadSyncMap(m *sync.Map) map[string]uint64 {
	result := make(map[string]uint64)

	m.Range(func(key, value any) bool {
		if v, ok := value.(*atomic.Uint64); ok {
			result[key.(string)] = v.Load()
		} else {
			result[key.(string)] = 0
		}
		return true
	})

	return result
}

// ExtractHTTPMethod extracts the HTTP method (GET, POST, PUT, DELETE) from the "uri" field
func ExtractHTTPMethod(uri string) string {
	if len(uri) == 0 {
		return "UNKNOWN"
	}
	parts := strings.Fields(uri)
	if len(parts) == 0 {
		return "UNKNOWN"
	}
	if len(parts) > 0 {
		method := parts[0]
		switch method {
		case "GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS", "PATCH":
			return method
		}
	}
	return "UNKNOWN"
}

// Clone creates a deep copy of the Metrics
func (m *Metrics) Clone() *Metrics {
	clone := NewMetrics(m.LatencyObs)

	clone.TotalRequests.Store(m.TotalRequests.Load())
	clone.BytesSent.Store(m.BytesSent.Load())
	clone.BytesReceived.Store(m.BytesReceived.Load())
	clone.Errors.Store(m.Errors.Load())

	copySyncMap(&m.RequestsDetailed, &clone.RequestsDetailed)
	copySyncMap(&m.RequestsByUser, &clone.RequestsByUser)
	copySyncMap(&m.RequestsByBucket, &clone.RequestsByBucket)
	copySyncMap(&m.RequestsByTenant, &clone.RequestsByTenant)
	copySyncMap(&m.RequestsByMethodDetailed, &clone.RequestsByMethodDetailed)
	copySyncMap(&m.RequestsByMethodPerUser, &clone.RequestsByMethodPerUser)
	copySyncMap(&m.RequestsByMethodPerBucket, &clone.RequestsByMethodPerBucket)
	copySyncMap(&m.RequestsByMethodPerTenant, &clone.RequestsByMethodPerTenant)
	copySyncMap(&m.RequestsByMethodGlobal, &clone.RequestsByMethodGlobal)
	copySyncMap(&m.RequestsByOperationDetailed, &clone.RequestsByOperationDetailed)
	copySyncMap(&m.RequestsByOperationPerUser, &clone.RequestsByOperationPerUser)
	copySyncMap(&m.RequestsByOperationPerBucket, &clone.RequestsByOperationPerBucket)
	copySyncMap(&m.RequestsByOperationPerTenant, &clone.RequestsByOperationPerTenant)
	copySyncMap(&m.RequestsByOperationGlobal, &clone.RequestsByOperationGlobal)
	copySyncMap(&m.RequestsByStatusDetailed, &clone.RequestsByStatusDetailed)
	copySyncMap(&m.RequestsByStatusPerUser, &clone.RequestsByStatusPerUser)
	copySyncMap(&m.RequestsByStatusPerBucket, &clone.RequestsByStatusPerBucket)
	copySyncMap(&m.RequestsByStatusPerTenant, &clone.RequestsByStatusPerTenant)
	copySyncMap(&m.RequestsPerStatusCode, &clone.RequestsPerStatusCode)
	copySyncMap(&m.BytesSentDetailed, &clone.BytesSentDetailed)
	copySyncMap(&m.BytesSentPerUser, &clone.BytesSentPerUser)
	copySyncMap(&m.BytesSentPerBucket, &clone.BytesSentPerBucket)
	copySyncMap(&m.BytesSentPerTenant, &clone.BytesSentPerTenant)
	copySyncMap(&m.BytesReceivedDetailed, &clone.BytesReceivedDetailed)
	copySyncMap(&m.BytesReceivedPerUser, &clone.BytesReceivedPerUser)
	copySyncMap(&m.BytesReceivedPerBucket, &clone.BytesReceivedPerBucket)
	copySyncMap(&m.BytesReceivedPerTenant, &clone.BytesReceivedPerTenant)
	copySyncMap(&m.ErrorsDetailed, &clone.ErrorsDetailed)
	copySyncMap(&m.ErrorsPerUser, &clone.ErrorsPerUser)
	copySyncMap(&m.ErrorsPerBucket, &clone.ErrorsPerBucket)
	copySyncMap(&m.ErrorsPerTenant, &clone.ErrorsPerTenant)
	copySyncMap(&m.ErrorsPerStatus, &clone.ErrorsPerStatus)
	copySyncMap(&m.ErrorsPerIP, &clone.ErrorsPerIP)
	copySyncMap(&m.TimeoutErrors, &clone.TimeoutErrors)
	copySyncMap(&m.ErrorsByCategory, &clone.ErrorsByCategory)
	copySyncMap(&m.RequestsByIPDetailed, &clone.RequestsByIPDetailed)
	copySyncMap(&m.RequestsPerIPPerTenant, &clone.RequestsPerIPPerTenant)
	copySyncMap(&m.RequestsPerTenantFromIP, &clone.RequestsPerTenantFromIP)
	copySyncMap(&m.RequestsByIPBucketMethodTenant, &clone.RequestsByIPBucketMethodTenant)
	copySyncMap(&m.BytesSentByIPDetailed, &clone.BytesSentByIPDetailed)
	copySyncMap(&m.BytesSentPerIPPerTenant, &clone.BytesSentPerIPPerTenant)
	copySyncMap(&m.BytesSentPerTenantFromIP, &clone.BytesSentPerTenantFromIP)
	copySyncMap(&m.BytesReceivedByIPDetailed, &clone.BytesReceivedByIPDetailed)
	copySyncMap(&m.BytesReceivedPerIPPerTenant, &clone.BytesReceivedPerIPPerTenant)
	copySyncMap(&m.BytesReceivedPerTenantFromIP, &clone.BytesReceivedPerTenantFromIP)

	return clone
}

// copySyncMap copies keys and atomic values from one sync.Map to another
func copySyncMap(src, dst *sync.Map) {
	src.Range(func(key, val any) bool {
		if orig, ok := val.(*atomic.Uint64); ok {
			var copied atomic.Uint64
			copied.Store(orig.Load())
			dst.Store(key, &copied)
		}
		return true
	})
}

// SubtractMetrics calculates the delta between two metrics objects: total - previous
func SubtractMetrics(total, previous *Metrics) *Metrics {
	delta := NewMetrics(total.LatencyObs)

	delta.TotalRequests.Store(diff(total.TotalRequests.Load(), previous.TotalRequests.Load()))
	delta.BytesSent.Store(diff(total.BytesSent.Load(), previous.BytesSent.Load()))
	delta.BytesReceived.Store(diff(total.BytesReceived.Load(), previous.BytesReceived.Load()))
	delta.Errors.Store(diff(total.Errors.Load(), previous.Errors.Load()))

	subtractSyncMap(&total.RequestsDetailed, &previous.RequestsDetailed, &delta.RequestsDetailed)
	subtractSyncMap(&total.RequestsByUser, &previous.RequestsByUser, &delta.RequestsByUser)
	subtractSyncMap(&total.RequestsByBucket, &previous.RequestsByBucket, &delta.RequestsByBucket)
	subtractSyncMap(&total.RequestsByTenant, &previous.RequestsByTenant, &delta.RequestsByTenant)
	subtractSyncMap(&total.RequestsByMethodDetailed, &previous.RequestsByMethodDetailed, &delta.RequestsByMethodDetailed)
	subtractSyncMap(&total.RequestsByMethodPerUser, &previous.RequestsByMethodPerUser, &delta.RequestsByMethodPerUser)
	subtractSyncMap(&total.RequestsByMethodPerBucket, &previous.RequestsByMethodPerBucket, &delta.RequestsByMethodPerBucket)
	subtractSyncMap(&total.RequestsByMethodPerTenant, &previous.RequestsByMethodPerTenant, &delta.RequestsByMethodPerTenant)
	subtractSyncMap(&total.RequestsByMethodGlobal, &previous.RequestsByMethodGlobal, &delta.RequestsByMethodGlobal)
	subtractSyncMap(&total.RequestsByOperationDetailed, &previous.RequestsByOperationDetailed, &delta.RequestsByOperationDetailed)
	subtractSyncMap(&total.RequestsByOperationPerUser, &previous.RequestsByOperationPerUser, &delta.RequestsByOperationPerUser)
	subtractSyncMap(&total.RequestsByOperationPerBucket, &previous.RequestsByOperationPerBucket, &delta.RequestsByOperationPerBucket)
	subtractSyncMap(&total.RequestsByOperationPerTenant, &previous.RequestsByOperationPerTenant, &delta.RequestsByOperationPerTenant)
	subtractSyncMap(&total.RequestsByOperationGlobal, &previous.RequestsByOperationGlobal, &delta.RequestsByOperationGlobal)
	subtractSyncMap(&total.RequestsByStatusDetailed, &previous.RequestsByStatusDetailed, &delta.RequestsByStatusDetailed)
	subtractSyncMap(&total.RequestsByStatusPerUser, &previous.RequestsByStatusPerUser, &delta.RequestsByStatusPerUser)
	subtractSyncMap(&total.RequestsByStatusPerBucket, &previous.RequestsByStatusPerBucket, &delta.RequestsByStatusPerBucket)
	subtractSyncMap(&total.RequestsByStatusPerTenant, &previous.RequestsByStatusPerTenant, &delta.RequestsByStatusPerTenant)
	subtractSyncMap(&total.RequestsPerStatusCode, &previous.RequestsPerStatusCode, &delta.RequestsPerStatusCode)
	subtractSyncMap(&total.BytesSentDetailed, &previous.BytesSentDetailed, &delta.BytesSentDetailed)
	subtractSyncMap(&total.BytesSentPerUser, &previous.BytesSentPerUser, &delta.BytesSentPerUser)
	subtractSyncMap(&total.BytesSentPerBucket, &previous.BytesSentPerBucket, &delta.BytesSentPerBucket)
	subtractSyncMap(&total.BytesSentPerTenant, &previous.BytesSentPerTenant, &delta.BytesSentPerTenant)
	subtractSyncMap(&total.BytesReceivedDetailed, &previous.BytesReceivedDetailed, &delta.BytesReceivedDetailed)
	subtractSyncMap(&total.BytesReceivedPerUser, &previous.BytesReceivedPerUser, &delta.BytesReceivedPerUser)
	subtractSyncMap(&total.BytesReceivedPerBucket, &previous.BytesReceivedPerBucket, &delta.BytesReceivedPerBucket)
	subtractSyncMap(&total.BytesReceivedPerTenant, &previous.BytesReceivedPerTenant, &delta.BytesReceivedPerTenant)
	subtractSyncMap(&total.ErrorsDetailed, &previous.ErrorsDetailed, &delta.ErrorsDetailed)
	subtractSyncMap(&total.ErrorsPerUser, &previous.ErrorsPerUser, &delta.ErrorsPerUser)
	subtractSyncMap(&total.ErrorsPerBucket, &previous.ErrorsPerBucket, &delta.ErrorsPerBucket)
	subtractSyncMap(&total.ErrorsPerTenant, &previous.ErrorsPerTenant, &delta.ErrorsPerTenant)
	subtractSyncMap(&total.ErrorsPerStatus, &previous.ErrorsPerStatus, &delta.ErrorsPerStatus)
	subtractSyncMap(&total.ErrorsPerIP, &previous.ErrorsPerIP, &delta.ErrorsPerIP)
	subtractSyncMap(&total.TimeoutErrors, &previous.TimeoutErrors, &delta.TimeoutErrors)
	subtractSyncMap(&total.ErrorsByCategory, &previous.ErrorsByCategory, &delta.ErrorsByCategory)
	subtractSyncMap(&total.RequestsByIPDetailed, &previous.RequestsByIPDetailed, &delta.RequestsByIPDetailed)
	subtractSyncMap(&total.RequestsPerIPPerTenant, &previous.RequestsPerIPPerTenant, &delta.RequestsPerIPPerTenant)
	subtractSyncMap(&total.RequestsPerTenantFromIP, &previous.RequestsPerTenantFromIP, &delta.RequestsPerTenantFromIP)
	subtractSyncMap(&total.RequestsByIPBucketMethodTenant, &previous.RequestsByIPBucketMethodTenant, &delta.RequestsByIPBucketMethodTenant)
	subtractSyncMap(&total.BytesSentByIPDetailed, &previous.BytesSentByIPDetailed, &delta.BytesSentByIPDetailed)
	subtractSyncMap(&total.BytesSentPerIPPerTenant, &previous.BytesSentPerIPPerTenant, &delta.BytesSentPerIPPerTenant)
	subtractSyncMap(&total.BytesSentPerTenantFromIP, &previous.BytesSentPerTenantFromIP, &delta.BytesSentPerTenantFromIP)
	subtractSyncMap(&total.BytesReceivedByIPDetailed, &previous.BytesReceivedByIPDetailed, &delta.BytesReceivedByIPDetailed)
	subtractSyncMap(&total.BytesReceivedPerIPPerTenant, &previous.BytesReceivedPerIPPerTenant, &delta.BytesReceivedPerIPPerTenant)
	subtractSyncMap(&total.BytesReceivedPerTenantFromIP, &previous.BytesReceivedPerTenantFromIP, &delta.BytesReceivedPerTenantFromIP)

	return delta
}

// subtractSyncMap calculates the difference and stores it in target
func subtractSyncMap(current, previous, target *sync.Map) {
	current.Range(func(key, curVal any) bool {
		cur := curVal.(*atomic.Uint64).Load()

		var prev uint64
		if prevVal, ok := previous.Load(key); ok {
			prev = prevVal.(*atomic.Uint64).Load()
		}

		delta := cur - prev
		if delta > 0 {
			var v atomic.Uint64
			v.Store(delta)
			target.Store(key, &v)
		}
		return true
	})
}

func diff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}
