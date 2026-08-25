package opslog

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubtractMetrics(t *testing.T) {
	total := NewMetrics()
	prev := NewMetrics()

	// Setup total values across different metric types
	total.TotalRequests.Store(10)
	total.BytesSent.Store(2048)

	// Test various storage maps
	total.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(5))
	total.RequestsByMethodDetailed.Store("user1|bucket1|GET", newUint64(3))
	total.BytesSentDetailed.Store("user1|bucket1", newUint64(1024))
	total.ErrorsDetailed.Store("user1|bucket1|404", newUint64(2))

	// Setup previous values
	prev.TotalRequests.Store(7)
	prev.BytesSent.Store(1024)
	prev.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(2))
	prev.BytesSentDetailed.Store("user1|bucket1", newUint64(512))

	// Subtract
	delta := SubtractMetrics(total, prev)

	// Test atomic counters
	assert.Equal(t, uint64(3), delta.TotalRequests.Load())
	assert.Equal(t, uint64(1024), delta.BytesSent.Load())

	// Test detailed requests delta
	v1, ok := delta.RequestsDetailed.Load("user1|bucket1|GET|200")
	assert.True(t, ok, "Expected key user1|bucket1|GET|200 to exist in RequestsDetailed")
	assert.Equal(t, uint64(3), v1.(*atomic.Uint64).Load())

	// Test method details (new key, should equal total)
	v2, ok := delta.RequestsByMethodDetailed.Load("user1|bucket1|GET")
	assert.True(t, ok, "Expected key user1|bucket1|GET to exist in RequestsByMethodDetailed")
	assert.Equal(t, uint64(3), v2.(*atomic.Uint64).Load())

	// Test bytes delta
	v3, ok := delta.BytesSentDetailed.Load("user1|bucket1")
	assert.True(t, ok, "Expected key user1|bucket1 to exist in BytesSentDetailed")
	assert.Equal(t, uint64(512), v3.(*atomic.Uint64).Load())

	// Test errors (new key, should equal total)
	v4, ok := delta.ErrorsDetailed.Load("user1|bucket1|404")
	assert.True(t, ok, "Expected key user1|bucket1|404 to exist in ErrorsDetailed")
	assert.Equal(t, uint64(2), v4.(*atomic.Uint64).Load())
}

func TestCloneMetrics(t *testing.T) {
	original := NewMetrics()

	// Set some base values across different metric types
	original.TotalRequests.Store(42)
	original.BytesSent.Store(1024)
	original.Errors.Store(5)

	// Test different storage maps
	original.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(5))
	original.RequestsByMethodDetailed.Store("user1|bucket1|GET", newUint64(3))
	original.BytesSentDetailed.Store("user1|bucket1", newUint64(1024))
	original.BytesSentPerUser.Store("user1", newUint64(888))
	original.ErrorsDetailed.Store("user1|bucket1|404", newUint64(2))
	original.RequestsByIPDetailed.Store("user1|192.168.1.1", newUint64(7))

	// Clone it
	clone := original.Clone()

	// Test top-level atomic fields
	assert.Equal(t, uint64(42), clone.TotalRequests.Load())
	assert.Equal(t, uint64(1024), clone.BytesSent.Load())
	assert.Equal(t, uint64(5), clone.Errors.Load())

	// Test sync.Map values across different types
	v1, ok := clone.RequestsDetailed.Load("user1|bucket1|GET|200")
	assert.True(t, ok, "Expected key to exist in RequestsDetailed")
	assert.Equal(t, uint64(5), v1.(*atomic.Uint64).Load())

	v2, ok := clone.RequestsByMethodDetailed.Load("user1|bucket1|GET")
	assert.True(t, ok, "Expected key to exist in RequestsByMethodDetailed")
	assert.Equal(t, uint64(3), v2.(*atomic.Uint64).Load())

	v3, ok := clone.BytesSentDetailed.Load("user1|bucket1")
	assert.True(t, ok, "Expected key to exist in BytesSentDetailed")
	assert.Equal(t, uint64(1024), v3.(*atomic.Uint64).Load())

	v4, ok := clone.BytesSentPerUser.Load("user1")
	assert.True(t, ok, "Expected key to exist in BytesSentPerUser")
	assert.Equal(t, uint64(888), v4.(*atomic.Uint64).Load())

	v5, ok := clone.ErrorsDetailed.Load("user1|bucket1|404")
	assert.True(t, ok, "Expected key to exist in ErrorsDetailed")
	assert.Equal(t, uint64(2), v5.(*atomic.Uint64).Load())

	v6, ok := clone.RequestsByIPDetailed.Load("user1|192.168.1.1")
	assert.True(t, ok, "Expected key to exist in RequestsByIPDetailed")
	assert.Equal(t, uint64(7), v6.(*atomic.Uint64).Load())

	// Mutate original, ensure clone is untouched
	original.TotalRequests.Add(10)
	original.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(99))

	// Verify clone remains unchanged
	assert.Equal(t, uint64(42), clone.TotalRequests.Load(), "Clone TotalRequests should remain unchanged")

	v1After, _ := clone.RequestsDetailed.Load("user1|bucket1|GET|200")
	assert.Equal(t, uint64(5), v1After.(*atomic.Uint64).Load(), "Clone RequestsDetailed should remain unchanged")
}

func TestSubtractMetrics_ZeroDelta(t *testing.T) {
	total := NewMetrics()
	prev := NewMetrics()

	// Test zero delta across different metric types
	total.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(5))
	prev.RequestsDetailed.Store("user1|bucket1|GET|200", newUint64(5))

	total.BytesSentPerUser.Store("user1", newUint64(1024))
	prev.BytesSentPerUser.Store("user1", newUint64(1024))

	delta := SubtractMetrics(total, prev)

	// Zero deltas should not be stored
	_, ok1 := delta.RequestsDetailed.Load("user1|bucket1|GET|200")
	assert.False(t, ok1, "Zero delta should not be stored in RequestsDetailed")

	_, ok2 := delta.BytesSentPerUser.Load("user1")
	assert.False(t, ok2, "Zero delta should not be stored in BytesSentPerUser")
}

func TestSubtractMetrics_MissingInPrev(t *testing.T) {
	total := NewMetrics()
	prev := NewMetrics()

	// Test new keys that don't exist in previous
	total.RequestsDetailed.Store("new|key|GET|200", newUint64(7))
	total.ErrorsPerUser.Store("newuser|404", newUint64(3))

	delta := SubtractMetrics(total, prev)

	// New keys should appear with full value
	v1, ok1 := delta.RequestsDetailed.Load("new|key|GET|200")
	assert.True(t, ok1, "New key should exist in delta")
	assert.Equal(t, uint64(7), v1.(*atomic.Uint64).Load())

	v2, ok2 := delta.ErrorsPerUser.Load("newuser|404")
	assert.True(t, ok2, "New error key should exist in delta")
	assert.Equal(t, uint64(3), v2.(*atomic.Uint64).Load())
}

func TestLatencyObsPropagation(t *testing.T) {
	called := false
	callCount := 0
	var capturedArgs []string

	cb := func(u, tnt, bucket, method string, sec float64) {
		called = true
		callCount++
		capturedArgs = []string{u, tnt, bucket, method}
		assert.Equal(t, "u1", u)
		assert.Equal(t, "t1", tnt)
		assert.Equal(t, "b1", bucket)
		assert.Equal(t, "M", method)
		assert.InDelta(t, 0.123, sec, 1e-6)
	}

	// Test direct call
	m := NewMetrics(cb)
	m.LatencyObs("u1", "t1", "b1", "M", 0.123)
	assert.True(t, called, "LatencyObs should be called")
	assert.Equal(t, 1, callCount)
	assert.Equal(t, []string{"u1", "t1", "b1", "M"}, capturedArgs)

	// Test clone carries it forward
	clone := m.Clone()
	called = false
	callCount = 0
	clone.LatencyObs("u1", "t1", "b1", "M", 0.123)
	assert.True(t, called, "Cloned LatencyObs should be called")
	assert.Equal(t, 1, callCount)

	// Test subtract carries it forward
	total := NewMetrics(cb)
	prev := NewMetrics(cb)
	delta := SubtractMetrics(total, prev)
	called = false
	callCount = 0
	delta.LatencyObs("u1", "t1", "b1", "M", 0.123)
	assert.True(t, called, "Delta LatencyObs should be called")
	assert.Equal(t, 1, callCount)
}

func TestLatencyObsDefaultNoOp(t *testing.T) {
	// Test that NewMetrics without callback creates no-op function
	m := NewMetrics()

	// Should not panic
	assert.NotPanics(t, func() {
		m.LatencyObs("user", "tenant", "bucket", "method", 1.23)
	}, "Default LatencyObs should be no-op and not panic")
}

func TestMetricsUpdate_BasicFunctionality(t *testing.T) {
	config := &MetricsConfig{
		TrackRequestsDetailed:  true,
		TrackLatencyDetailed:   true,
		TrackBytesSentDetailed: true,
		TrackErrorsDetailed:    true,
	}

	latencyCallCount := 0
	latencyObs := func(user, tenant, bucket, method string, seconds float64) {
		latencyCallCount++
		assert.Equal(t, "user1", user)
		assert.Equal(t, "tenant1", tenant)
		assert.Equal(t, "bucket1", bucket)
		assert.Equal(t, "GET", method)
		assert.InDelta(t, 0.150, seconds, 1e-6) // 150ms converted to seconds
	}

	m := NewMetrics(latencyObs)

	logEntry := S3OperationLog{
		User:          "user1$tenant1",
		Bucket:        "bucket1",
		URI:           "GET /bucket1/object.txt HTTP/1.1",
		HTTPStatus:    "200",
		BytesSent:     1024,
		BytesReceived: 0,
		TotalTime:     150, // milliseconds
	}

	// Update metrics
	m.Update(logEntry, config)

	// Verify atomic counters
	assert.Equal(t, uint64(1), m.TotalRequests.Load())
	assert.Equal(t, uint64(1024), m.BytesSent.Load())
	assert.Equal(t, uint64(0), m.BytesReceived.Load())
	assert.Equal(t, uint64(0), m.Errors.Load()) // 200 is not an error

	// Verify detailed requests tracking
	v, ok := m.RequestsDetailed.Load("user1$tenant1|bucket1|GET|200")
	assert.True(t, ok, "Should track detailed request")
	assert.Equal(t, uint64(1), v.(*atomic.Uint64).Load())

	// Verify bytes tracking
	v2, ok2 := m.BytesSentDetailed.Load("user1$tenant1|bucket1")
	assert.True(t, ok2, "Should track detailed bytes sent")
	assert.Equal(t, uint64(1024), v2.(*atomic.Uint64).Load())

	// Verify latency observation was called
	assert.Equal(t, 1, latencyCallCount, "LatencyObs should be called once")
}

func TestClassifySLIOperation(t *testing.T) {
	testCases := []struct {
		name      string
		operation string
		expected  SLIOperation
		ok        bool
	}{
		// GET — object retrieval (includes HEAD-on-object, which RGW logs as get_obj)
		{name: "get_obj", operation: "get_obj", expected: SLIOperationGet, ok: true},

		// PUT — object writes
		{name: "put_obj", operation: "put_obj", expected: SLIOperationPut, ok: true},
		{name: "post_obj", operation: "post_obj", expected: SLIOperationPut, ok: true},
		{name: "copy_obj", operation: "copy_obj", expected: SLIOperationPut, ok: true},
		{name: "restore_obj", operation: "restore_obj", expected: SLIOperationPut, ok: true},
		{name: "bulk_upload", operation: "bulk_upload", expected: SLIOperationPut, ok: true},

		// DELETE — object removal
		{name: "delete_obj", operation: "delete_obj", expected: SLIOperationDelete, ok: true},
		{name: "multi_object_delete", operation: "multi_object_delete", expected: SLIOperationDelete, ok: true},
		{name: "bulk_delete", operation: "bulk_delete", expected: SLIOperationDelete, ok: true},

		// LIST — bucket and account listing
		{name: "list_bucket", operation: "list_bucket", expected: SLIOperationList, ok: true},
		{name: "list_buckets", operation: "list_buckets", expected: SLIOperationList, ok: true},

		// HEAD — metadata-only stat operations
		{name: "stat_bucket", operation: "stat_bucket", expected: SLIOperationHead, ok: true},
		{name: "stat_account", operation: "stat_account", expected: SLIOperationHead, ok: true},

		// MULTIPART — multipart upload lifecycle
		{name: "init_multipart", operation: "init_multipart", expected: SLIOperationMultipart, ok: true},
		{name: "complete_multipart", operation: "complete_multipart", expected: SLIOperationMultipart, ok: true},
		{name: "abort_multipart", operation: "abort_multipart", expected: SLIOperationMultipart, ok: true},
		{name: "list_multipart", operation: "list_multipart", expected: SLIOperationMultipart, ok: true},
		{name: "list_bucket_multiparts", operation: "list_bucket_multiparts", expected: SLIOperationMultipart, ok: true},

		// Control-plane operations — deliberately excluded from SLI
		{name: "create_bucket excluded", operation: "create_bucket", expected: "", ok: false},
		{name: "delete_bucket excluded", operation: "delete_bucket", expected: "", ok: false},
		{name: "put_bucket_acl excluded", operation: "put_acls", expected: "", ok: false},
		{name: "get_acls excluded", operation: "get_acls", expected: "", ok: false},
		{name: "get_bucket_info excluded", operation: "get_bucket_info", expected: "", ok: false},

		// Case insensitivity
		{name: "case insensitive", operation: "GET_OBJ", expected: SLIOperationGet, ok: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := classifySLIOperation(tc.operation)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestDetectProtocol(t *testing.T) {
	testCases := []struct {
		name     string
		uri      string
		expected SLIProtocol
	}{
		// S3 operations — no /swift/v1/ in URI
		{name: "s3 get", uri: "GET /bucket/key HTTP/1.1", expected: SLIProtocolS3},
		{name: "s3 put", uri: "PUT /bucket/key HTTP/1.1", expected: SLIProtocolS3},
		{name: "s3 list", uri: "GET /bucket?list-type=2 HTTP/1.1", expected: SLIProtocolS3},

		// Swift operations — detected from /swift/v1/ in URI
		{name: "swift list", uri: "GET /swift/v1/AUTH_tenant/container?limit=30 HTTP/1.1", expected: SLIProtocolSwift},
		{name: "swift get", uri: "GET /swift/v1/AUTH_tenant/container/object HTTP/1.1", expected: SLIProtocolSwift},
		{name: "swift put", uri: "PUT /swift/v1/AUTH_tenant/container/object HTTP/1.1", expected: SLIProtocolSwift},
		{name: "swift head", uri: "HEAD /swift/v1/AUTH_tenant/container/object HTTP/1.1", expected: SLIProtocolSwift},
		{name: "swift delete", uri: "DELETE /swift/v1/AUTH_tenant/container/object HTTP/1.1", expected: SLIProtocolSwift},
		{name: "swift account listing", uri: "GET /swift/v1/AUTH_tenant HTTP/1.1", expected: SLIProtocolSwift},

		// Edge cases
		{name: "empty uri defaults to s3", uri: "", expected: SLIProtocolS3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := detectProtocol(tc.uri)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestStatusClass(t *testing.T) {
	testCases := []struct {
		name     string
		status   string
		expected string
	}{
		{name: "success", status: "200", expected: "2xx"},
		{name: "client error", status: "404", expected: "4xx"},
		{name: "server error", status: "503", expected: "5xx"},
		{name: "informational", status: "100", expected: "1xx"},
		{name: "redirect", status: "301", expected: "3xx"},
		{name: "empty", status: "", expected: "unknown"},
		{name: "alpha", status: "ok", expected: "unknown"},
		{name: "leading whitespace", status: " 200", expected: "unknown"},
		{name: "too short", status: "20", expected: "unknown"},
		{name: "single digit", status: "2", expected: "unknown"},
		{name: "too long", status: "2000", expected: "unknown"},
		{name: "leading zero", status: "099", expected: "unknown"},
		{name: "six hundred", status: "600", expected: "unknown"},
		{name: "nine hundred", status: "999", expected: "unknown"},
		{name: "non-digit second char", status: "2x0", expected: "unknown"},
		{name: "non-digit third char", status: "20x", expected: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, statusClass(tc.status))
		})
	}
}

func TestMetricsUpdate_TrackBucketSLO(t *testing.T) {
	// Set up a test collector (bypass prometheus registration)
	prev := globalSLICollector
	globalSLICollector = newSLICollector(SLICollectorConfig{
		StaleTTL: 24 * time.Hour,
	})
	t.Cleanup(func() { globalSLICollector = prev })

	config := &MetricsConfig{TrackBucketSLO: true}
	logEntry := S3OperationLog{
		Bucket:     "bucket-slo-test",
		User:       "alice$tenant-slo-test",
		Operation:  "get_obj",
		HTTPStatus: "200",
		TotalTime:  150,
	}

	// Counter is now keyed by tenant+protocol+operation+status_class (no bucket)
	beforeCounter := globalSLICollector.counterValue("tenant-slo-test", "s3", "get", "2xx")
	beforeLatency := globalSLICollector.latencyCount("tenant-slo-test", "s3", "get")
	assert.NotPanics(t, func() {
		NewMetrics().Update(logEntry, config)
	})
	afterCounter := globalSLICollector.counterValue("tenant-slo-test", "s3", "get", "2xx")
	afterLatency := globalSLICollector.latencyCount("tenant-slo-test", "s3", "get")

	assert.Equal(t, beforeCounter+1, afterCounter, "SLI counter should increment")
	assert.Equal(t, beforeLatency+1, afterLatency, "SLI latency histogram should record observation when TotalTime > 0")
}

func TestMetricsUpdate_TrackBucketSLO_SwiftFromURI(t *testing.T) {
	// Verify that the SLI pipeline correctly detects Swift protocol from the
	// URI field when the operation name has no swift_ prefix (production
	// JSON ops-log format produced by rgw_ops_log_file_path).
	prev := globalSLICollector
	globalSLICollector = newSLICollector(SLICollectorConfig{
		StaleTTL: 24 * time.Hour,
	})
	t.Cleanup(func() { globalSLICollector = prev })

	config := &MetricsConfig{TrackBucketSLO: true}
	logEntry := S3OperationLog{
		Bucket:     "mailbox-incoming",
		User:       "alice$tenant-swift-uri",
		Operation:  "list_bucket",
		URI:        "GET /swift/v1/AUTH_tenant-swift-uri/mailbox-incoming?limit=30 HTTP/1.1",
		HTTPStatus: "200",
		TotalTime:  1,
	}

	beforeCounter := globalSLICollector.counterValue("tenant-swift-uri", "swift", "list", "2xx")
	beforeLatency := globalSLICollector.latencyCount("tenant-swift-uri", "swift", "list")
	NewMetrics().Update(logEntry, config)
	afterCounter := globalSLICollector.counterValue("tenant-swift-uri", "swift", "list", "2xx")
	afterLatency := globalSLICollector.latencyCount("tenant-swift-uri", "swift", "list")

	assert.Equal(t, beforeCounter+1, afterCounter,
		"Swift request (detected from URI) should be counted under protocol=swift")
	assert.Equal(t, beforeLatency+1, afterLatency,
		"Swift request (detected from URI) should record latency under protocol=swift")

	// Verify it was NOT counted under s3
	s3Counter := globalSLICollector.counterValue("tenant-swift-uri", "s3", "list", "2xx")
	assert.Equal(t, float64(0), s3Counter,
		"Swift request should not appear under protocol=s3")
}

func TestMetricsUpdate_TrackBucketSLO_ZeroLatency(t *testing.T) {
	// Set up a test collector
	prev := globalSLICollector
	globalSLICollector = newSLICollector(SLICollectorConfig{
		StaleTTL: 24 * time.Hour,
	})
	t.Cleanup(func() { globalSLICollector = prev })

	config := &MetricsConfig{TrackBucketSLO: true}
	logEntry := S3OperationLog{
		Bucket:     "bucket-slo-zero",
		User:       "alice$tenant-slo-zero",
		Operation:  "get_obj",
		HTTPStatus: "200",
		TotalTime:  0, // sub-ms or missing timing
	}

	beforeCounter := globalSLICollector.counterValue("tenant-slo-zero", "s3", "get", "2xx")
	beforeLatency := globalSLICollector.latencyCount("tenant-slo-zero", "s3", "get")
	NewMetrics().Update(logEntry, config)
	afterCounter := globalSLICollector.counterValue("tenant-slo-zero", "s3", "get", "2xx")
	afterLatency := globalSLICollector.latencyCount("tenant-slo-zero", "s3", "get")

	assert.Equal(t, beforeCounter+1, afterCounter, "SLI counter should increment even with zero latency")
	assert.Equal(t, beforeLatency+1, afterLatency, "SLI latency histogram should record TotalTime=0 (sub-ms) in le=0.05 bucket")
}

func TestSLICollector_StaleSeriesNotEmitted(t *testing.T) {
	collector := newSLICollector(SLICollectorConfig{
		StaleTTL: 1 * time.Millisecond, // very short for testing
	})
	prev := globalSLICollector
	globalSLICollector = collector
	t.Cleanup(func() { globalSLICollector = prev })

	// Observe a request
	collector.observeCounter("t1", "s3", "get", "2xx")

	// Verify series exists before going stale
	countersBefore := collector.collectCounterMetrics()
	sliBefore := findCounterByLabels(countersBefore, "t1", "s3", "get", "2xx")
	assert.NotNil(t, sliBefore, "SLI counter should be emitted while active")

	// Wait for the series to become stale
	time.Sleep(5 * time.Millisecond)

	// Collect — stale series should not be emitted
	countersAfter := collector.collectCounterMetrics()
	sliAfter := findCounterByLabels(countersAfter, "t1", "s3", "get", "2xx")
	assert.Nil(t, sliAfter, "Stale SLI counter should not be emitted")
}

func TestSLICollector_Reap(t *testing.T) {
	collector := newSLICollector(SLICollectorConfig{
		StaleTTL: 1 * time.Millisecond,
	})
	prev := globalSLICollector
	globalSLICollector = collector
	t.Cleanup(func() { globalSLICollector = prev })

	// Observe requests for multiple operations
	collector.observeCounter("t1", "s3", "get", "2xx")
	collector.observeCounter("t1", "swift", "list", "2xx")

	counters := collector.seriesCount()
	assert.Equal(t, 2, counters, "Should have 2 counter series")

	// Wait for series to become stale, then reap
	time.Sleep(5 * time.Millisecond)
	collector.reap()

	counters = collector.seriesCount()
	assert.Equal(t, 0, counters, "Reaped counters should be 0")
	assert.Equal(t, uint64(2), collector.reapedTotal.Load(), "Should report 2 reaped series")
}

func TestMetricsUpdate_ErrorTracking(t *testing.T) {
	config := &MetricsConfig{
		TrackErrorsDetailed: true,
		TrackErrorsPerUser:  true,
	}

	m := NewMetrics()

	logEntry := S3OperationLog{
		User:       "user1$tenant1",
		Bucket:     "bucket1",
		URI:        "GET /bucket1/missing.txt HTTP/1.1",
		HTTPStatus: "404",
	}

	m.Update(logEntry, config)

	// Verify error counters
	assert.Equal(t, uint64(1), m.Errors.Load())

	// Verify detailed error tracking
	v1, ok1 := m.ErrorsDetailed.Load("user1$tenant1|bucket1|404")
	assert.True(t, ok1, "Should track detailed error")
	assert.Equal(t, uint64(1), v1.(*atomic.Uint64).Load())

	// Verify per-user error tracking
	v2, ok2 := m.ErrorsPerUser.Load("user1|404")
	assert.True(t, ok2, "Should track per-user error")
	assert.Equal(t, uint64(1), v2.(*atomic.Uint64).Load())
}

func TestMetricsUpdate_ConditionalTracking(t *testing.T) {
	// Test that disabled tracking doesn't create entries
	config := &MetricsConfig{
		TrackRequestsDetailed: false,
		TrackErrorsDetailed:   false,
		TrackLatencyDetailed:  false,
	}

	m := NewMetrics()

	logEntry := S3OperationLog{
		User:       "user1$tenant1",
		Bucket:     "bucket1",
		URI:        "GET /bucket1/object.txt HTTP/1.1",
		HTTPStatus: "404",
		TotalTime:  150,
	}

	m.Update(logEntry, config)

	// Basic counters should still work
	assert.Equal(t, uint64(1), m.TotalRequests.Load())
	assert.Equal(t, uint64(1), m.Errors.Load())

	// But detailed tracking should be empty
	_, ok1 := m.RequestsDetailed.Load("user1$tenant1|bucket1|GET|404")
	assert.False(t, ok1, "Should not track detailed requests when disabled")

	_, ok2 := m.ErrorsDetailed.Load("user1$tenant1|bucket1|404")
	assert.False(t, ok2, "Should not track detailed errors when disabled")
}

func newUint64(val uint64) *atomic.Uint64 {
	var u atomic.Uint64
	u.Store(val)
	return &u
}
