// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

// AuditSinkConfig defines the RabbitMQ audit sink configuration.
type AuditSinkConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	RabbitMQURL string `mapstructure:"rabbitmq_url"`
	// RabbitMQUsername and RabbitMQPassword, when set, are composed into the
	// RabbitMQURL userinfo at runtime, overriding any credentials embedded in
	// the URL. This lets the username and password be supplied as two separate
	// values (e.g. two Vault entries synced into a Secret) instead of being
	// baked into a single connection string.
	RabbitMQUsername  string `mapstructure:"rabbitmq_username"`
	RabbitMQPassword  string `mapstructure:"rabbitmq_password"`
	QueueName         string `mapstructure:"queue_name"`
	InternalQueueSize int    `mapstructure:"internal_queue_size"` // Optional, defaults to 20
	Debug             bool   `mapstructure:"debug"`               // Log published events
	// RequireTenant drops audit events that carry neither a project_id nor a
	// domain_id before publishing (the audit consumer rejects such events).
	RequireTenant bool `mapstructure:"require_tenant"`
	// Region is a static per-cluster value stamped onto each audit event's
	// target (the ops log has no region). Empty means not stamped.
	Region string `mapstructure:"region"`
	// ObserverName is the CADF observer name identifying the storage service in
	// emitted events (e.g. radosgw/ceph/swift). Empty defaults to "radosgw".
	ObserverName string `mapstructure:"observer_name"`
	// IncludeReads controls whether read operations (get/head/list) are audited.
	// Default true: object-storage audit includes data-access events (reads) as
	// well as mutations (cf. GCS data-access logs). Set false for mutations-only.
	IncludeReads bool `mapstructure:"include_reads"`
	// SkipBuckets is a comma-separated, case-insensitive list of bucket names
	// excluded from audit. It breaks the Hermes loop: Hermes writes audit events
	// into a (WORM) bucket, and auditing those writes would re-trigger events.
	// Defaults to "hermes" via the flag; empty disables the filter.
	SkipBuckets string `mapstructure:"skip_buckets"`
	// AllowDomains and DenyDomains scope the audit trail to specific Keystone
	// domains, reducing the volume published to RabbitMQ. Both are
	// comma-separated, case-insensitive lists; each token is matched against the
	// entry's project domain ID *and* name (so either form works). Matching is on
	// KeystoneScope.Project.Domain. Precedence: an entry whose domain is in
	// DenyDomains is always dropped; then, if AllowDomains is non-empty, only
	// entries whose domain is in it are kept. Both empty = audit all domains.
	AllowDomains string `mapstructure:"allow_domains"`
	DenyDomains  string `mapstructure:"deny_domains"`
}

type OpsLogConfig struct {
	LogFilePath               string
	TruncateLogOnStart        bool
	SocketPath                string
	NatsURL                   string
	NatsSubject               string
	NatsMetricsSubject        string
	UseNats                   bool
	LogToStdout               bool
	LogPrettyPrint            bool
	LogRetentionDays          int   // Number of days to keep old log files
	MaxLogFileSize            int64 // Maximum log file size in bytes before rotation
	Prometheus                bool
	PrometheusPort            int
	PodName                   string
	Region                    string // Deployment region for SLI metric labels (e.g. "eu-de-1")
	IgnoreAnonymousRequests   bool
	PrometheusIntervalSeconds int
	MetricsConfig             MetricsConfig
	AuditSink                 AuditSinkConfig
}

// MetricsConfig defines which metrics to collect and at what granularity
type MetricsConfig struct {
	// === SHORTCUT CONFIGS ===
	TrackEverything bool `yaml:"track_everything"` // Enables all metrics at all levels
	TrackBucketSLO  bool `yaml:"track_bucket_slo"` // Dedicated low-cardinality per-tenant GET/LIST SLI metrics for Prometheus SLOs

	// === BUCKET SLO COLLECTOR SETTINGS ===
	BucketSLOStaleTTL     string `yaml:"bucket_slo_stale_ttl"`     // Duration after which idle tenant series are reaped (default: 24h)
	BucketSLOReapInterval string `yaml:"bucket_slo_reap_interval"` // How often the reaper runs (default: stale_ttl/4)

	// === REQUEST METRICS ===
	// Total requests
	TrackRequestsDetailed  bool `yaml:"track_requests_detailed"`   // Full detail: pod, user, tenant, bucket, method, http_status
	TrackRequestsPerUser   bool `yaml:"track_requests_per_user"`   // Aggregated: pod, user, tenant, method, http_status
	TrackRequestsPerBucket bool `yaml:"track_requests_per_bucket"` // Aggregated: pod, tenant, bucket, method, http_status
	TrackRequestsPerTenant bool `yaml:"track_requests_per_tenant"` // Aggregated: pod, tenant, method, http_status

	// Method-based requests
	TrackRequestsByMethodDetailed  bool `yaml:"track_requests_by_method"`            // Detailed: pod, user, tenant, bucket, method
	TrackRequestsByMethodPerUser   bool `yaml:"track_requests_by_method_per_user"`   // Aggregated: pod, user, tenant, method
	TrackRequestsByMethodPerBucket bool `yaml:"track_requests_by_method_per_bucket"` // Aggregated: pod, tenant, bucket, method
	TrackRequestsByMethodPerTenant bool `yaml:"track_requests_by_method_per_tenant"` // Aggregated: pod, tenant, method
	TrackRequestsByMethodGlobal    bool `yaml:"track_requests_by_method_global"`     // Aggregated: pod, method

	// Operation-based requests
	TrackRequestsByOperationDetailed  bool `yaml:"track_requests_by_operation"`            // Detailed: pod, user, tenant, bucket, operation, method
	TrackRequestsByOperationPerUser   bool `yaml:"track_requests_by_operation_per_user"`   // Aggregated: pod, user, tenant, operation, method
	TrackRequestsByOperationPerBucket bool `yaml:"track_requests_by_operation_per_bucket"` // Aggregated: pod, tenant, bucket, operation, method
	TrackRequestsByOperationPerTenant bool `yaml:"track_requests_by_operation_per_tenant"` // Aggregated: pod, tenant, operation, method
	TrackRequestsByOperationGlobal    bool `yaml:"track_requests_by_operation_global"`     // Aggregated: pod, operation, method

	// Status-based requests
	TrackRequestsByStatusDetailed  bool `yaml:"track_requests_by_status_detailed"`   // Detailed: pod, user, tenant, bucket, status
	TrackRequestsByStatusPerUser   bool `yaml:"track_requests_by_status_per_user"`   // Aggregated: pod, user, tenant, status
	TrackRequestsByStatusPerBucket bool `yaml:"track_requests_by_status_per_bucket"` // Aggregated: pod, tenant, bucket, status
	TrackRequestsByStatusPerTenant bool `yaml:"track_requests_by_status_per_tenant"` // Aggregated: pod, tenant, status

	// === BYTES METRICS ===
	// Bytes sent
	TrackBytesSentDetailed  bool `yaml:"track_bytes_sent_detailed"`   // Detailed: pod, user, tenant, bucket
	TrackBytesSentPerUser   bool `yaml:"track_bytes_sent_per_user"`   // Aggregated: pod, user, tenant
	TrackBytesSentPerBucket bool `yaml:"track_bytes_sent_per_bucket"` // Aggregated: pod, tenant, bucket
	TrackBytesSentPerTenant bool `yaml:"track_bytes_sent_per_tenant"` // Aggregated: pod, tenant

	// Bytes received
	TrackBytesReceivedDetailed  bool `yaml:"track_bytes_received_detailed"`   // Detailed: pod, user, tenant, bucket
	TrackBytesReceivedPerUser   bool `yaml:"track_bytes_received_per_user"`   // Aggregated: pod, user, tenant
	TrackBytesReceivedPerBucket bool `yaml:"track_bytes_received_per_bucket"` // Aggregated: pod, tenant, bucket
	TrackBytesReceivedPerTenant bool `yaml:"track_bytes_received_per_tenant"` // Aggregated: pod, tenant

	// === ERROR METRICS ===
	// Errors
	TrackErrorsDetailed   bool `yaml:"track_errors_detailed"`    // Detailed: pod, user, tenant, bucket, http_status
	TrackErrorsPerUser    bool `yaml:"track_errors_per_user"`    // Aggregated: pod, user, tenant, http_status
	TrackErrorsPerBucket  bool `yaml:"track_errors_per_bucket"`  // Aggregated: pod, tenant, bucket, http_status
	TrackErrorsPerTenant  bool `yaml:"track_errors_per_tenant"`  // Aggregated: pod, tenant, http_status
	TrackErrorsPerStatus  bool `yaml:"track_errors_per_status"`  // Aggregated: pod, http_status
	TrackErrorsByIP       bool `yaml:"track_errors_by_ip"`       // IP-based: pod, ip, tenant, http_status
	TrackTimeoutErrors    bool `yaml:"track_timeout_errors"`     // Timeout-specific: pod, user, tenant, bucket, timeout_type
	TrackErrorsByCategory bool `yaml:"track_errors_by_category"` // Categorized: pod, tenant, bucket, error_category, http_status

	// === IP-BASED METRICS ===
	// Requests by IP
	TrackRequestsByIPDetailed           bool `yaml:"track_requests_by_ip"`                      // Detailed: pod, user, tenant, ip
	TrackRequestsByIPPerTenant          bool `yaml:"track_requests_by_ip_per_tenant"`           // Aggregated: pod, tenant, ip
	TrackRequestsByIPBucketMethodTenant bool `yaml:"track_requests_by_ip_bucket_method_tenant"` // Detailed: pod, ip, bucket, method, tenant
	TrackRequestsByIPGlobalPerTenant    bool `yaml:"track_requests_by_ip_global_per_tenant"`    // Aggregated: pod, tenant

	// Bytes by IP
	TrackBytesSentByIPDetailed        bool `yaml:"track_bytes_sent_by_ip"`                   // Detailed: pod, user, tenant, ip
	TrackBytesSentByIPPerTenant       bool `yaml:"track_bytes_sent_by_ip_per_tenant"`        // Aggregated: pod, tenant, ip
	TrackBytesSentByIPGlobalPerTenant bool `yaml:"track_bytes_sent_by_ip_global_per_tenant"` // Aggregated: pod, tenant

	TrackBytesReceivedByIPDetailed        bool `yaml:"track_bytes_received_by_ip"`                   // Detailed: pod, user, tenant, ip
	TrackBytesReceivedByIPPerTenant       bool `yaml:"track_bytes_received_by_ip_per_tenant"`        // Aggregated: pod, tenant, ip
	TrackBytesReceivedByIPGlobalPerTenant bool `yaml:"track_bytes_received_by_ip_global_per_tenant"` // Aggregated: pod, tenant

	// === LATENCY METRICS ===
	TrackLatencyDetailed           bool `yaml:"track_latency_detailed"`              // Detailed: user, tenant, bucket, method (no pod!)
	TrackLatencyPerUser            bool `yaml:"track_latency_per_user"`              // Aggregated: user, tenant, method
	TrackLatencyPerBucket          bool `yaml:"track_latency_per_bucket"`            // Aggregated: tenant, bucket, method
	TrackLatencyPerTenant          bool `yaml:"track_latency_per_tenant"`            // Aggregated: tenant, method
	TrackLatencyPerMethod          bool `yaml:"track_latency_per_method"`            // Aggregated: method
	TrackLatencyPerBucketAndMethod bool `yaml:"track_latency_per_bucket_and_method"` // Aggregated: tenant, bucket, method
}

// ApplyShortcuts applies shortcut configurations
func (c *MetricsConfig) ApplyShortcuts() {
	if c.TrackEverything {
		// Enable only detailed metrics - aggregations can be done in Prometheus queries
		// This is the most efficient approach with lowest cardinality
		c.TrackBucketSLO = true
		c.TrackRequestsDetailed = true
		c.TrackRequestsByMethodDetailed = true
		c.TrackRequestsByOperationDetailed = true
		c.TrackRequestsByStatusDetailed = true

		c.TrackBytesSentDetailed = true
		c.TrackBytesReceivedDetailed = true

		c.TrackErrorsDetailed = true
		c.TrackErrorsByIP = true
		c.TrackTimeoutErrors = true
		c.TrackErrorsByCategory = true

		c.TrackRequestsByIPDetailed = true
		c.TrackBytesSentByIPDetailed = true
		c.TrackBytesReceivedByIPDetailed = true

		c.TrackLatencyDetailed = true
	}
}
