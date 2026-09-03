// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"
	"os"

	"github.com/cobaltcore-dev/prysm/pkg/producers/opslog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	opsLogFilePath             string
	opsTruncateLogOnStart      bool
	opsSocketPath              string
	opsNatsURL                 string
	opsNatsSubject             string
	opsNatsMetricsSubject      string
	opsLogToStdout             bool
	opsLogPrettyPrint          bool
	opsLogRetentionDays        int
	opsMaxLogFileSize          int64
	opsPromEnabled             bool
	opsPromPort                int
	opsIgnoreAnonymousRequests bool
	opsPromIntervalSeconds     int

	// Audit flags
	opsAuditEnabled           bool
	opsAuditRabbitMQURL       string
	opsAuditRabbitMQUsername  string
	opsAuditRabbitMQPassword  string
	opsAuditQueueName         string
	opsAuditInternalQueueSize int
	opsAuditDebug             bool
	opsAuditRequireTenant     bool
	opsAuditRegion            string
	opsAuditObserverName      string
	opsAuditIncludeReads      bool
	opsAuditSkipBuckets       string
	opsAuditAllowDomains      string
	opsAuditDenyDomains       string

	// Shortcut config
	opsTrackEverything bool
	opsTrackBucketSLO  bool

	// Request metrics flags
	opsTrackRequestsDetailed  bool
	opsTrackRequestsPerUser   bool
	opsTrackRequestsPerBucket bool
	opsTrackRequestsPerTenant bool

	// Method-based request flags
	opsTrackRequestsByMethodDetailed  bool
	opsTrackRequestsByMethodPerUser   bool
	opsTrackRequestsByMethodPerBucket bool
	opsTrackRequestsByMethodPerTenant bool
	opsTrackRequestsByMethodGlobal    bool

	// Operation-based request flags
	opsTrackRequestsByOperationDetailed  bool
	opsTrackRequestsByOperationPerUser   bool
	opsTrackRequestsByOperationPerBucket bool
	opsTrackRequestsByOperationPerTenant bool
	opsTrackRequestsByOperationGlobal    bool

	// Status-based request flags
	opsTrackRequestsByStatusDetailed  bool
	opsTrackRequestsByStatusPerUser   bool
	opsTrackRequestsByStatusPerBucket bool
	opsTrackRequestsByStatusPerTenant bool

	// Bytes metrics flags
	opsTrackBytesSentDetailed  bool
	opsTrackBytesSentPerUser   bool
	opsTrackBytesSentPerBucket bool
	opsTrackBytesSentPerTenant bool

	opsTrackBytesReceivedDetailed  bool
	opsTrackBytesReceivedPerUser   bool
	opsTrackBytesReceivedPerBucket bool
	opsTrackBytesReceivedPerTenant bool

	// Error metrics flags
	opsTrackErrorsDetailed   bool
	opsTrackErrorsPerUser    bool
	opsTrackErrorsPerBucket  bool
	opsTrackErrorsPerTenant  bool
	opsTrackErrorsPerStatus  bool
	opsTrackErrorsByIP       bool
	opsTrackTimeoutErrors    bool
	opsTrackErrorsByCategory bool

	// IP-based metrics flags
	opsTrackRequestsByIPDetailed           bool
	opsTrackRequestsByIPPerTenant          bool
	opsTrackRequestsByIPBucketMethodTenant bool
	opsTrackRequestsByIPGlobalPerTenant    bool

	opsTrackBytesSentByIPDetailed        bool
	opsTrackBytesSentByIPPerTenant       bool
	opsTrackBytesSentByIPGlobalPerTenant bool

	opsTrackBytesReceivedByIPDetailed        bool
	opsTrackBytesReceivedByIPPerTenant       bool
	opsTrackBytesReceivedByIPGlobalPerTenant bool

	// Latency metrics flags
	opsTrackLatencyDetailed           bool
	opsTrackLatencyPerUser            bool
	opsTrackLatencyPerBucket          bool
	opsTrackLatencyPerTenant          bool
	opsTrackLatencyPerMethod          bool
	opsTrackLatencyPerBucketAndMethod bool
)

var opsLogCmd = &cobra.Command{
	Use:   "ops-log",
	Short: "Start the S3 operations logger",
	Long: `Start the S3 operations logger.

Note: Before using this command, ensure that RGW is configured to log S3 operations with the necessary details.

To enable RGW ops log to file feature, run the following commands:

  # ceph config set global rgw_ops_log_rados false
  # ceph config set global rgw_ops_log_file_path '/var/log/ceph/ops-log-$cluster-$name.log'
  # ceph config set global rgw_enable_ops_log true

Then restart all RadosGW daemons:

  # ceph orch ps
  # ceph orch daemon restart <rgw>

Following this configuration change, the RadosGW will log operations to the file /var/log/ceph/ceph-rgw-ops.json.log.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := opslog.OpsLogConfig{
			LogFilePath:               opsLogFilePath,
			TruncateLogOnStart:        opsTruncateLogOnStart,
			SocketPath:                opsSocketPath,
			NatsURL:                   opsNatsURL,
			NatsSubject:               opsNatsSubject,
			NatsMetricsSubject:        opsNatsMetricsSubject,
			LogToStdout:               opsLogToStdout,
			LogPrettyPrint:            opsLogPrettyPrint,
			LogRetentionDays:          opsLogRetentionDays,
			MaxLogFileSize:            opsMaxLogFileSize,
			Prometheus:                opsPromEnabled,
			PrometheusPort:            opsPromPort,
			IgnoreAnonymousRequests:   opsIgnoreAnonymousRequests,
			PrometheusIntervalSeconds: opsPromIntervalSeconds,
			MetricsConfig: opslog.MetricsConfig{
				// Shortcut config
				TrackEverything: opsTrackEverything,
				TrackBucketSLO:  opsTrackBucketSLO,

				// Request metrics
				TrackRequestsDetailed:  opsTrackRequestsDetailed,
				TrackRequestsPerUser:   opsTrackRequestsPerUser,
				TrackRequestsPerBucket: opsTrackRequestsPerBucket,
				TrackRequestsPerTenant: opsTrackRequestsPerTenant,

				// Method-based requests
				TrackRequestsByMethodDetailed:  opsTrackRequestsByMethodDetailed,
				TrackRequestsByMethodPerUser:   opsTrackRequestsByMethodPerUser,
				TrackRequestsByMethodPerBucket: opsTrackRequestsByMethodPerBucket,
				TrackRequestsByMethodPerTenant: opsTrackRequestsByMethodPerTenant,
				TrackRequestsByMethodGlobal:    opsTrackRequestsByMethodGlobal,

				// Operation-based requests
				TrackRequestsByOperationDetailed:  opsTrackRequestsByOperationDetailed,
				TrackRequestsByOperationPerUser:   opsTrackRequestsByOperationPerUser,
				TrackRequestsByOperationPerBucket: opsTrackRequestsByOperationPerBucket,
				TrackRequestsByOperationPerTenant: opsTrackRequestsByOperationPerTenant,
				TrackRequestsByOperationGlobal:    opsTrackRequestsByOperationGlobal,

				// Status-based requests
				TrackRequestsByStatusDetailed:  opsTrackRequestsByStatusDetailed,
				TrackRequestsByStatusPerUser:   opsTrackRequestsByStatusPerUser,
				TrackRequestsByStatusPerBucket: opsTrackRequestsByStatusPerBucket,
				TrackRequestsByStatusPerTenant: opsTrackRequestsByStatusPerTenant,

				// Bytes metrics
				TrackBytesSentDetailed:  opsTrackBytesSentDetailed,
				TrackBytesSentPerUser:   opsTrackBytesSentPerUser,
				TrackBytesSentPerBucket: opsTrackBytesSentPerBucket,
				TrackBytesSentPerTenant: opsTrackBytesSentPerTenant,

				TrackBytesReceivedDetailed:  opsTrackBytesReceivedDetailed,
				TrackBytesReceivedPerUser:   opsTrackBytesReceivedPerUser,
				TrackBytesReceivedPerBucket: opsTrackBytesReceivedPerBucket,
				TrackBytesReceivedPerTenant: opsTrackBytesReceivedPerTenant,

				// Error metrics
				TrackErrorsDetailed:   opsTrackErrorsDetailed,
				TrackErrorsPerUser:    opsTrackErrorsPerUser,
				TrackErrorsPerBucket:  opsTrackErrorsPerBucket,
				TrackErrorsPerTenant:  opsTrackErrorsPerTenant,
				TrackErrorsPerStatus:  opsTrackErrorsPerStatus,
				TrackTimeoutErrors:    opsTrackTimeoutErrors,
				TrackErrorsByCategory: opsTrackErrorsByCategory,

				// IP-based metrics
				TrackRequestsByIPDetailed:           opsTrackRequestsByIPDetailed,
				TrackRequestsByIPPerTenant:          opsTrackRequestsByIPPerTenant,
				TrackRequestsByIPBucketMethodTenant: opsTrackRequestsByIPBucketMethodTenant,
				TrackRequestsByIPGlobalPerTenant:    opsTrackRequestsByIPGlobalPerTenant,

				TrackBytesSentByIPDetailed:        opsTrackBytesSentByIPDetailed,
				TrackBytesSentByIPPerTenant:       opsTrackBytesSentByIPPerTenant,
				TrackBytesSentByIPGlobalPerTenant: opsTrackBytesSentByIPGlobalPerTenant,

				TrackBytesReceivedByIPDetailed:        opsTrackBytesReceivedByIPDetailed,
				TrackBytesReceivedByIPPerTenant:       opsTrackBytesReceivedByIPPerTenant,
				TrackBytesReceivedByIPGlobalPerTenant: opsTrackBytesReceivedByIPGlobalPerTenant,

				TrackErrorsByIP: opsTrackErrorsByIP,

				// Latency metrics
				TrackLatencyDetailed:           opsTrackLatencyDetailed,
				TrackLatencyPerUser:            opsTrackLatencyPerUser,
				TrackLatencyPerBucket:          opsTrackLatencyPerBucket,
				TrackLatencyPerTenant:          opsTrackLatencyPerTenant,
				TrackLatencyPerMethod:          opsTrackLatencyPerMethod,
				TrackLatencyPerBucketAndMethod: opsTrackLatencyPerBucketAndMethod,
			},
			AuditSink: opslog.AuditSinkConfig{
				Enabled:           opsAuditEnabled,
				RabbitMQURL:       opsAuditRabbitMQURL,
				RabbitMQUsername:  opsAuditRabbitMQUsername,
				RabbitMQPassword:  opsAuditRabbitMQPassword,
				QueueName:         opsAuditQueueName,
				InternalQueueSize: opsAuditInternalQueueSize,
				Debug:             opsAuditDebug,
				RequireTenant:     opsAuditRequireTenant,
				Region:            opsAuditRegion,
				ObserverName:      opsAuditObserverName,
				IncludeReads:      opsAuditIncludeReads,
				SkipBuckets:       opsAuditSkipBuckets,
				AllowDomains:      opsAuditAllowDomains,
				DenyDomains:       opsAuditDenyDomains,
			},
		}

		config = mergeOpsLogConfigWithEnv(config)

		config.UseNats = config.NatsURL != ""

		event := log.Info()
		event.Bool("use_nats", config.UseNats)
		if config.UseNats {
			event.Str("nats_url", config.NatsURL)
			event.Str("nats_subject", config.NatsSubject)
			event.Str("nats_metrics_subject", config.NatsMetricsSubject)
		}

		if config.LogFilePath != "" {
			event.Str("log_file_path", config.LogFilePath)
		}

		if config.SocketPath != "" {
			event.Str("socket_path", config.SocketPath)
		}

		if config.LogToStdout {
			event.Bool("log_to_stdout", config.LogToStdout)
		}

		if config.LogPrettyPrint {
			event.Bool("log_pretty_print", config.LogPrettyPrint)
		}

		event.Int("log_retention_days", config.LogRetentionDays)
		event.Int64("max_log_file_size", config.MaxLogFileSize)

		event.Bool("prometheus_enabled", config.Prometheus)
		if config.Prometheus {
			event.Int("prometheus_port", config.PrometheusPort)
		}

		// Enhanced debugging for tracking options
		debugTrackingConfig(event, config.MetricsConfig)

		event.Msg("OpsLog configuration initialized")

		event.Msg("OpsLog configuration initialized")

		validateOpsLogConfig(config)

		if config.SocketPath != "" {
			opslog.StartSocketOpsLogger(config)
		} else {
			opslog.StartFileOpsLogger(config)
		}
	},
}

// debugTrackingConfig adds comprehensive metrics configuration to the zerolog event
func debugTrackingConfig(event *zerolog.Event, config opslog.MetricsConfig) {
	// Count enabled metrics for summary
	totalEnabled := 0

	// Shortcut configuration
	event.Bool("track_everything", config.TrackEverything)
	if config.TrackEverything {
		event.Str("memory_usage", "high").Str("note", "all metrics enabled")
		return // Don't add individual flags if everything is enabled
	}

	if config.TrackBucketSLO {
		event.Bool("track_bucket_slo", true)
		totalEnabled++
	}

	// Request tracking
	requestMetrics := []string{}
	if config.TrackRequestsDetailed {
		requestMetrics = append(requestMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackRequestsPerUser {
		requestMetrics = append(requestMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackRequestsPerBucket {
		requestMetrics = append(requestMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackRequestsPerTenant {
		requestMetrics = append(requestMetrics, "per-tenant")
		totalEnabled++
	}
	if len(requestMetrics) > 0 {
		event.Strs("request_tracking", requestMetrics)
	}

	// Method-based tracking
	methodMetrics := []string{}
	if config.TrackRequestsByMethodDetailed {
		methodMetrics = append(methodMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackRequestsByMethodPerUser {
		methodMetrics = append(methodMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackRequestsByMethodPerBucket {
		methodMetrics = append(methodMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackRequestsByMethodPerTenant {
		methodMetrics = append(methodMetrics, "per-tenant")
		totalEnabled++
	}
	if config.TrackRequestsByMethodGlobal {
		methodMetrics = append(methodMetrics, "global")
		totalEnabled++
	}
	if len(methodMetrics) > 0 {
		event.Strs("method_tracking", methodMetrics)
	}

	// Operation-based tracking
	operationMetrics := []string{}
	if config.TrackRequestsByOperationDetailed {
		operationMetrics = append(operationMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackRequestsByOperationPerUser {
		operationMetrics = append(operationMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackRequestsByOperationPerBucket {
		operationMetrics = append(operationMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackRequestsByOperationPerTenant {
		operationMetrics = append(operationMetrics, "per-tenant")
		totalEnabled++
	}
	if config.TrackRequestsByOperationGlobal {
		operationMetrics = append(operationMetrics, "global")
		totalEnabled++
	}
	if len(operationMetrics) > 0 {
		event.Strs("operation_tracking", operationMetrics)
	}

	// Status-based tracking
	statusMetrics := []string{}
	if config.TrackRequestsByStatusDetailed {
		statusMetrics = append(statusMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackRequestsByStatusPerUser {
		statusMetrics = append(statusMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackRequestsByStatusPerBucket {
		statusMetrics = append(statusMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackRequestsByStatusPerTenant {
		statusMetrics = append(statusMetrics, "per-tenant")
		totalEnabled++
	}
	if len(statusMetrics) > 0 {
		event.Strs("status_tracking", statusMetrics)
	}

	// Bytes tracking
	bytesMetrics := []string{}
	if config.TrackBytesSentDetailed {
		bytesMetrics = append(bytesMetrics, "sent-detailed")
		totalEnabled++
	}
	if config.TrackBytesSentPerUser {
		bytesMetrics = append(bytesMetrics, "sent-per-user")
		totalEnabled++
	}
	if config.TrackBytesSentPerBucket {
		bytesMetrics = append(bytesMetrics, "sent-per-bucket")
		totalEnabled++
	}
	if config.TrackBytesSentPerTenant {
		bytesMetrics = append(bytesMetrics, "sent-per-tenant")
		totalEnabled++
	}
	if config.TrackBytesReceivedDetailed {
		bytesMetrics = append(bytesMetrics, "received-detailed")
		totalEnabled++
	}
	if config.TrackBytesReceivedPerUser {
		bytesMetrics = append(bytesMetrics, "received-per-user")
		totalEnabled++
	}
	if config.TrackBytesReceivedPerBucket {
		bytesMetrics = append(bytesMetrics, "received-per-bucket")
		totalEnabled++
	}
	if config.TrackBytesReceivedPerTenant {
		bytesMetrics = append(bytesMetrics, "received-per-tenant")
		totalEnabled++
	}
	if len(bytesMetrics) > 0 {
		event.Strs("bytes_tracking", bytesMetrics)
	}

	// Error tracking
	errorMetrics := []string{}
	if config.TrackErrorsDetailed {
		errorMetrics = append(errorMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackErrorsPerUser {
		errorMetrics = append(errorMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackErrorsPerBucket {
		errorMetrics = append(errorMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackErrorsPerTenant {
		errorMetrics = append(errorMetrics, "per-tenant")
		totalEnabled++
	}
	if config.TrackErrorsPerStatus {
		errorMetrics = append(errorMetrics, "per-status")
		totalEnabled++
	}
	if config.TrackErrorsByIP {
		errorMetrics = append(errorMetrics, "by-ip")
		totalEnabled++
	}
	if len(errorMetrics) > 0 {
		event.Strs("error_tracking", errorMetrics)
	}

	// IP-based tracking
	ipMetrics := []string{}
	if config.TrackRequestsByIPDetailed {
		ipMetrics = append(ipMetrics, "requests-detailed")
		totalEnabled++
	}
	if config.TrackRequestsByIPPerTenant {
		ipMetrics = append(ipMetrics, "requests-per-tenant")
		totalEnabled++
	}
	if config.TrackRequestsByIPBucketMethodTenant {
		ipMetrics = append(ipMetrics, "requests-bucket-method-tenant")
		totalEnabled++
	}
	if config.TrackRequestsByIPGlobalPerTenant {
		ipMetrics = append(ipMetrics, "requests-global-per-tenant")
		totalEnabled++
	}
	if config.TrackBytesSentByIPDetailed {
		ipMetrics = append(ipMetrics, "bytes-sent-detailed")
		totalEnabled++
	}
	if config.TrackBytesSentByIPPerTenant {
		ipMetrics = append(ipMetrics, "bytes-sent-per-tenant")
		totalEnabled++
	}
	if config.TrackBytesSentByIPGlobalPerTenant {
		ipMetrics = append(ipMetrics, "bytes-sent-global-per-tenant")
		totalEnabled++
	}
	if config.TrackBytesReceivedByIPDetailed {
		ipMetrics = append(ipMetrics, "bytes-received-detailed")
		totalEnabled++
	}
	if config.TrackBytesReceivedByIPPerTenant {
		ipMetrics = append(ipMetrics, "bytes-received-per-tenant")
		totalEnabled++
	}
	if config.TrackBytesReceivedByIPGlobalPerTenant {
		ipMetrics = append(ipMetrics, "bytes-received-global-per-tenant")
		totalEnabled++
	}
	if len(ipMetrics) > 0 {
		event.Strs("ip_tracking", ipMetrics)
	}

	// Latency tracking (uses histograms, not storage maps)
	latencyMetrics := []string{}
	if config.TrackLatencyDetailed {
		latencyMetrics = append(latencyMetrics, "detailed")
		totalEnabled++
	}
	if config.TrackLatencyPerUser {
		latencyMetrics = append(latencyMetrics, "per-user")
		totalEnabled++
	}
	if config.TrackLatencyPerBucket {
		latencyMetrics = append(latencyMetrics, "per-bucket")
		totalEnabled++
	}
	if config.TrackLatencyPerTenant {
		latencyMetrics = append(latencyMetrics, "per-tenant")
		totalEnabled++
	}
	if config.TrackLatencyPerMethod {
		latencyMetrics = append(latencyMetrics, "per-method")
		totalEnabled++
	}
	if config.TrackLatencyPerBucketAndMethod {
		latencyMetrics = append(latencyMetrics, "per-bucket-and-method")
		totalEnabled++
	}
	if len(latencyMetrics) > 0 {
		event.Strs("latency_tracking", latencyMetrics)
	}

	// Summary information
	event.Int("total_enabled_metrics", totalEnabled)

	// Memory efficiency classification
	if totalEnabled == 0 {
		event.Str("memory_usage", "minimal").Str("note", "only basic counters")
	} else if totalEnabled > 30 {
		event.Str("memory_usage", "high").Str("note", "consider reducing in production")
	} else {
		event.Str("memory_usage", "efficient").Str("architecture", "dedicated storage")
	}
}

func mergeOpsLogConfigWithEnv(cfg opslog.OpsLogConfig) opslog.OpsLogConfig {
	cfg.LogFilePath = getEnv("LOG_FILE_PATH", cfg.LogFilePath)
	cfg.TruncateLogOnStart = getEnvBool("TRUNCATE_LOG_ON_START", cfg.TruncateLogOnStart)
	cfg.SocketPath = getEnv("SOCKET_PATH", cfg.SocketPath)
	cfg.NatsURL = getEnv("NATS_URL", cfg.NatsURL)
	cfg.NatsSubject = getEnv("NATS_SUBJECT", cfg.NatsSubject)
	cfg.NatsMetricsSubject = getEnv("NATS_METRICS_SUBJECT", cfg.NatsMetricsSubject)
	cfg.LogToStdout = getEnvBool("LOG_TO_STDOUT", cfg.LogToStdout)
	cfg.LogPrettyPrint = getEnvBool("LOG_PRETTY_PRINT", cfg.LogPrettyPrint)
	cfg.LogRetentionDays = getEnvInt("LOG_RETENTION_DAYS", cfg.LogRetentionDays)
	cfg.MaxLogFileSize = getEnvInt64("MAX_LOG_FILE_SIZE", cfg.MaxLogFileSize)
	cfg.PrometheusPort = getEnvInt("PROMETHEUS_PORT", cfg.PrometheusPort)
	cfg.PodName = getEnv("POD_NAME", cfg.PodName)
	cfg.IgnoreAnonymousRequests = getEnvBool("IGNORE_ANONYMOUS_REQUESTS", cfg.IgnoreAnonymousRequests)
	cfg.PrometheusIntervalSeconds = getEnvInt("PROMETHEUS_INTERVAL", cfg.PrometheusIntervalSeconds)

	// Shortcut config
	cfg.MetricsConfig.TrackEverything = getEnvBool("TRACK_EVERYTHING", cfg.MetricsConfig.TrackEverything)
	cfg.MetricsConfig.TrackBucketSLO = getEnvBool("TRACK_BUCKET_SLO", cfg.MetricsConfig.TrackBucketSLO)

	// Request metrics environment variables
	cfg.MetricsConfig.TrackRequestsDetailed = getEnvBool("TRACK_REQUESTS_DETAILED", cfg.MetricsConfig.TrackRequestsDetailed)
	cfg.MetricsConfig.TrackRequestsPerUser = getEnvBool("TRACK_REQUESTS_PER_USER", cfg.MetricsConfig.TrackRequestsPerUser)
	cfg.MetricsConfig.TrackRequestsPerBucket = getEnvBool("TRACK_REQUESTS_PER_BUCKET", cfg.MetricsConfig.TrackRequestsPerBucket)
	cfg.MetricsConfig.TrackRequestsPerTenant = getEnvBool("TRACK_REQUESTS_PER_TENANT", cfg.MetricsConfig.TrackRequestsPerTenant)

	// Method-based requests
	cfg.MetricsConfig.TrackRequestsByMethodDetailed = getEnvBool("TRACK_REQUESTS_BY_METHOD_DETAILED", cfg.MetricsConfig.TrackRequestsByMethodDetailed)
	cfg.MetricsConfig.TrackRequestsByMethodPerUser = getEnvBool("TRACK_REQUESTS_BY_METHOD_PER_USER", cfg.MetricsConfig.TrackRequestsByMethodPerUser)
	cfg.MetricsConfig.TrackRequestsByMethodPerBucket = getEnvBool("TRACK_REQUESTS_BY_METHOD_PER_BUCKET", cfg.MetricsConfig.TrackRequestsByMethodPerBucket)
	cfg.MetricsConfig.TrackRequestsByMethodPerTenant = getEnvBool("TRACK_REQUESTS_BY_METHOD_PER_TENANT", cfg.MetricsConfig.TrackRequestsByMethodPerTenant)
	cfg.MetricsConfig.TrackRequestsByMethodGlobal = getEnvBool("TRACK_REQUESTS_BY_METHOD_GLOBAL", cfg.MetricsConfig.TrackRequestsByMethodGlobal)

	// Operation-based requests
	cfg.MetricsConfig.TrackRequestsByOperationDetailed = getEnvBool("TRACK_REQUESTS_BY_OPERATION_DETAILED", cfg.MetricsConfig.TrackRequestsByOperationDetailed)
	cfg.MetricsConfig.TrackRequestsByOperationPerUser = getEnvBool("TRACK_REQUESTS_BY_OPERATION_PER_USER", cfg.MetricsConfig.TrackRequestsByOperationPerUser)
	cfg.MetricsConfig.TrackRequestsByOperationPerBucket = getEnvBool("TRACK_REQUESTS_BY_OPERATION_PER_BUCKET", cfg.MetricsConfig.TrackRequestsByOperationPerBucket)
	cfg.MetricsConfig.TrackRequestsByOperationPerTenant = getEnvBool("TRACK_REQUESTS_BY_OPERATION_PER_TENANT", cfg.MetricsConfig.TrackRequestsByOperationPerTenant)
	cfg.MetricsConfig.TrackRequestsByOperationGlobal = getEnvBool("TRACK_REQUESTS_BY_OPERATION_GLOBAL", cfg.MetricsConfig.TrackRequestsByOperationGlobal)

	// Status-based requests
	cfg.MetricsConfig.TrackRequestsByStatusDetailed = getEnvBool("TRACK_REQUESTS_BY_STATUS_DETAILED", cfg.MetricsConfig.TrackRequestsByStatusDetailed)
	cfg.MetricsConfig.TrackRequestsByStatusPerUser = getEnvBool("TRACK_REQUESTS_BY_STATUS_PER_USER", cfg.MetricsConfig.TrackRequestsByStatusPerUser)
	cfg.MetricsConfig.TrackRequestsByStatusPerBucket = getEnvBool("TRACK_REQUESTS_BY_STATUS_PER_BUCKET", cfg.MetricsConfig.TrackRequestsByStatusPerBucket)
	cfg.MetricsConfig.TrackRequestsByStatusPerTenant = getEnvBool("TRACK_REQUESTS_BY_STATUS_PER_TENANT", cfg.MetricsConfig.TrackRequestsByStatusPerTenant)

	// Bytes metrics
	cfg.MetricsConfig.TrackBytesSentDetailed = getEnvBool("TRACK_BYTES_SENT_DETAILED", cfg.MetricsConfig.TrackBytesSentDetailed)
	cfg.MetricsConfig.TrackBytesSentPerUser = getEnvBool("TRACK_BYTES_SENT_PER_USER", cfg.MetricsConfig.TrackBytesSentPerUser)
	cfg.MetricsConfig.TrackBytesSentPerBucket = getEnvBool("TRACK_BYTES_SENT_PER_BUCKET", cfg.MetricsConfig.TrackBytesSentPerBucket)
	cfg.MetricsConfig.TrackBytesSentPerTenant = getEnvBool("TRACK_BYTES_SENT_PER_TENANT", cfg.MetricsConfig.TrackBytesSentPerTenant)

	cfg.MetricsConfig.TrackBytesReceivedDetailed = getEnvBool("TRACK_BYTES_RECEIVED_DETAILED", cfg.MetricsConfig.TrackBytesReceivedDetailed)
	cfg.MetricsConfig.TrackBytesReceivedPerUser = getEnvBool("TRACK_BYTES_RECEIVED_PER_USER", cfg.MetricsConfig.TrackBytesReceivedPerUser)
	cfg.MetricsConfig.TrackBytesReceivedPerBucket = getEnvBool("TRACK_BYTES_RECEIVED_PER_BUCKET", cfg.MetricsConfig.TrackBytesReceivedPerBucket)
	cfg.MetricsConfig.TrackBytesReceivedPerTenant = getEnvBool("TRACK_BYTES_RECEIVED_PER_TENANT", cfg.MetricsConfig.TrackBytesReceivedPerTenant)

	// Error metrics
	cfg.MetricsConfig.TrackErrorsDetailed = getEnvBool("TRACK_ERRORS_DETAILED", cfg.MetricsConfig.TrackErrorsDetailed)
	cfg.MetricsConfig.TrackErrorsPerUser = getEnvBool("TRACK_ERRORS_PER_USER", cfg.MetricsConfig.TrackErrorsPerUser)
	cfg.MetricsConfig.TrackErrorsPerBucket = getEnvBool("TRACK_ERRORS_PER_BUCKET", cfg.MetricsConfig.TrackErrorsPerBucket)
	cfg.MetricsConfig.TrackErrorsPerTenant = getEnvBool("TRACK_ERRORS_PER_TENANT", cfg.MetricsConfig.TrackErrorsPerTenant)
	cfg.MetricsConfig.TrackErrorsPerStatus = getEnvBool("TRACK_ERRORS_PER_STATUS", cfg.MetricsConfig.TrackErrorsPerStatus)
	cfg.MetricsConfig.TrackErrorsByIP = getEnvBool("TRACK_ERRORS_BY_IP", cfg.MetricsConfig.TrackErrorsByIP)
	cfg.MetricsConfig.TrackTimeoutErrors = getEnvBool("TRACK_TIMEOUT_ERRORS", cfg.MetricsConfig.TrackTimeoutErrors)
	cfg.MetricsConfig.TrackErrorsByCategory = getEnvBool("TRACK_ERRORS_BY_CATEGORY", cfg.MetricsConfig.TrackErrorsByCategory)

	// IP-based metrics
	cfg.MetricsConfig.TrackRequestsByIPDetailed = getEnvBool("TRACK_REQUESTS_BY_IP_DETAILED", cfg.MetricsConfig.TrackRequestsByIPDetailed)
	cfg.MetricsConfig.TrackRequestsByIPPerTenant = getEnvBool("TRACK_REQUESTS_BY_IP_PER_TENANT", cfg.MetricsConfig.TrackRequestsByIPPerTenant)
	cfg.MetricsConfig.TrackRequestsByIPBucketMethodTenant = getEnvBool("TRACK_REQUESTS_BY_IP_BUCKET_METHOD_TENANT", cfg.MetricsConfig.TrackRequestsByIPBucketMethodTenant)
	cfg.MetricsConfig.TrackRequestsByIPGlobalPerTenant = getEnvBool("TRACK_REQUESTS_BY_IP_GLOBAL_PER_TENANT", cfg.MetricsConfig.TrackRequestsByIPGlobalPerTenant)

	cfg.MetricsConfig.TrackBytesSentByIPDetailed = getEnvBool("TRACK_BYTES_SENT_BY_IP_DETAILED", cfg.MetricsConfig.TrackBytesSentByIPDetailed)
	cfg.MetricsConfig.TrackBytesSentByIPPerTenant = getEnvBool("TRACK_BYTES_SENT_BY_IP_PER_TENANT", cfg.MetricsConfig.TrackBytesSentByIPPerTenant)
	cfg.MetricsConfig.TrackBytesSentByIPGlobalPerTenant = getEnvBool("TRACK_BYTES_SENT_BY_IP_GLOBAL_PER_TENANT", cfg.MetricsConfig.TrackBytesSentByIPGlobalPerTenant)

	cfg.MetricsConfig.TrackBytesReceivedByIPDetailed = getEnvBool("TRACK_BYTES_RECEIVED_BY_IP_DETAILED", cfg.MetricsConfig.TrackBytesReceivedByIPDetailed)
	cfg.MetricsConfig.TrackBytesReceivedByIPPerTenant = getEnvBool("TRACK_BYTES_RECEIVED_BY_IP_PER_TENANT", cfg.MetricsConfig.TrackBytesReceivedByIPPerTenant)
	cfg.MetricsConfig.TrackBytesReceivedByIPGlobalPerTenant = getEnvBool("TRACK_BYTES_RECEIVED_BY_IP_GLOBAL_PER_TENANT", cfg.MetricsConfig.TrackBytesReceivedByIPGlobalPerTenant)

	// Latency metrics
	cfg.MetricsConfig.TrackLatencyDetailed = getEnvBool("TRACK_LATENCY_DETAILED", cfg.MetricsConfig.TrackLatencyDetailed)
	cfg.MetricsConfig.TrackLatencyPerUser = getEnvBool("TRACK_LATENCY_PER_USER", cfg.MetricsConfig.TrackLatencyPerUser)
	cfg.MetricsConfig.TrackLatencyPerBucket = getEnvBool("TRACK_LATENCY_PER_BUCKET", cfg.MetricsConfig.TrackLatencyPerBucket)
	cfg.MetricsConfig.TrackLatencyPerTenant = getEnvBool("TRACK_LATENCY_PER_TENANT", cfg.MetricsConfig.TrackLatencyPerTenant)
	cfg.MetricsConfig.TrackLatencyPerMethod = getEnvBool("TRACK_LATENCY_PER_METHOD", cfg.MetricsConfig.TrackLatencyPerMethod)
	cfg.MetricsConfig.TrackLatencyPerBucketAndMethod = getEnvBool("TRACK_LATENCY_PER_BUCKET_AND_METHOD", cfg.MetricsConfig.TrackLatencyPerBucketAndMethod)

	// Audit sink (RabbitMQ) configuration. These mirror the --audit-* flags so
	// the sink can be enabled via env vars injected by the mutating webhook
	// (Secret/ConfigMap) without editing the sidecar command line.
	cfg.AuditSink.Enabled = getEnvBool("AUDIT_ENABLED", cfg.AuditSink.Enabled)
	cfg.AuditSink.RabbitMQURL = getEnv("AUDIT_RABBITMQ_URL", cfg.AuditSink.RabbitMQURL)
	cfg.AuditSink.RabbitMQUsername = getEnv("AUDIT_RABBITMQ_USERNAME", cfg.AuditSink.RabbitMQUsername)
	cfg.AuditSink.RabbitMQPassword = getEnv("AUDIT_RABBITMQ_PASSWORD", cfg.AuditSink.RabbitMQPassword)
	cfg.AuditSink.QueueName = getEnv("AUDIT_QUEUE_NAME", cfg.AuditSink.QueueName)
	cfg.AuditSink.RequireTenant = getEnvBool("AUDIT_REQUIRE_TENANT", cfg.AuditSink.RequireTenant)
	cfg.AuditSink.Region = getEnv("AUDIT_REGION", cfg.AuditSink.Region)
	cfg.AuditSink.ObserverName = getEnv("AUDIT_OBSERVER_NAME", cfg.AuditSink.ObserverName)
	cfg.AuditSink.IncludeReads = getEnvBool("AUDIT_INCLUDE_READS", cfg.AuditSink.IncludeReads)
	cfg.AuditSink.SkipBuckets = getEnv("AUDIT_SKIP_BUCKETS", cfg.AuditSink.SkipBuckets)
	cfg.AuditSink.AllowDomains = getEnv("AUDIT_ALLOW_DOMAINS", cfg.AuditSink.AllowDomains)
	cfg.AuditSink.DenyDomains = getEnv("AUDIT_DENY_DOMAINS", cfg.AuditSink.DenyDomains)
	cfg.AuditSink.InternalQueueSize = getEnvInt("AUDIT_QUEUE_SIZE", cfg.AuditSink.InternalQueueSize)
	cfg.AuditSink.Debug = getEnvBool("AUDIT_DEBUG", cfg.AuditSink.Debug)

	return cfg
}

func init() {
	opsLogCmd.Flags().StringVar(&opsLogFilePath, "log-file", "/var/log/ceph/ceph-rgw-ops.json.log", "Path to the S3 operations log file")
	opsLogCmd.Flags().BoolVar(&opsTruncateLogOnStart, "truncate-log-on-start", true, "Truncate ops log file at startup to avoid duplicate processing")
	opsLogCmd.Flags().StringVar(&opsSocketPath, "socket-path", "", "Path to the Unix domain socket")
	opsLogCmd.Flags().StringVar(&opsNatsURL, "nats-url", "", "NATS server URL")
	opsLogCmd.Flags().StringVar(&opsNatsSubject, "nats-subject", "rgw.s3.ops", "NATS subject to publish results")
	opsLogCmd.Flags().StringVar(&opsNatsMetricsSubject, "nats-metrics-subject", "rgw.s3.ops.aggregated.metrics", "NATS subject to publish aggregated metrics")
	opsLogCmd.Flags().BoolVar(&opsLogToStdout, "log-to-stdout", false, "Log operations to stdout instead of a file")
	opsLogCmd.Flags().BoolVar(&opsLogPrettyPrint, "log-pretty-print", false, "Enable pretty printing for log output")
	opsLogCmd.Flags().IntVar(&opsLogRetentionDays, "log-retention-days", 1, "Number of days to retain old log files")
	opsLogCmd.Flags().Int64Var(&opsMaxLogFileSize, "max-log-file-size", 10, "Maximum log file size in MB before rotation (e.g., 10 for 10 MB)")
	opsLogCmd.Flags().BoolVar(&opsPromEnabled, "prometheus", false, "Enable Prometheus metrics")
	opsLogCmd.Flags().IntVar(&opsPromPort, "prometheus-port", 8080, "Prometheus metrics port")
	opsLogCmd.Flags().BoolVar(&opsIgnoreAnonymousRequests, "ignore-anonymous-requests", true, "Ignore anonymous requests (must remain enabled when --track-bucket-slo is used to prevent tenant='none' from polluting SLI metrics)")
	opsLogCmd.Flags().IntVar(&opsPromIntervalSeconds, "prometheus-interval", 60, "Prometheus metrics update interval in seconds")

	// Audit flags
	opsLogCmd.Flags().BoolVar(&opsAuditEnabled, "audit-enabled", false, "Enable audit event publishing to RabbitMQ")
	opsLogCmd.Flags().StringVar(&opsAuditRabbitMQURL, "audit-rabbitmq-url", "", "RabbitMQ connection URL (amqp://host:port); credentials may be embedded or supplied via --audit-rabbitmq-username/--audit-rabbitmq-password")
	opsLogCmd.Flags().StringVar(&opsAuditRabbitMQUsername, "audit-rabbitmq-username", "", "RabbitMQ username; overrides any userinfo in --audit-rabbitmq-url (e.g. sourced from a Vault entry)")
	opsLogCmd.Flags().StringVar(&opsAuditRabbitMQPassword, "audit-rabbitmq-password", "", "RabbitMQ password; overrides any userinfo in --audit-rabbitmq-url (e.g. sourced from a Vault entry)")
	opsLogCmd.Flags().StringVar(&opsAuditQueueName, "audit-queue-name", "keystone.notifications.info", "RabbitMQ queue name for audit events")
	opsLogCmd.Flags().IntVar(&opsAuditInternalQueueSize, "audit-queue-size", 20, "Internal queue size for audit events")
	opsLogCmd.Flags().BoolVar(&opsAuditDebug, "audit-debug", false, "Log published audit events for debugging")
	opsLogCmd.Flags().BoolVar(&opsAuditRequireTenant, "audit-require-tenant", true, "Drop audit events that have neither a project_id nor a domain_id (the audit consumer rejects them)")
	opsLogCmd.Flags().StringVar(&opsAuditRegion, "audit-region", "", "Static region stamped onto each audit event (the ops log has none); empty = not stamped")
	opsLogCmd.Flags().StringVar(&opsAuditObserverName, "audit-observer-name", "radosgw", "CADF observer name identifying the storage service in audit events (e.g. radosgw/ceph/swift)")
	opsLogCmd.Flags().BoolVar(&opsAuditIncludeReads, "audit-include-reads", true, "Audit read operations (get_/head_/stat_/list_ prefixed ops); default true for object-storage data-access auditing. Set false for mutations-only")
	opsLogCmd.Flags().StringVar(&opsAuditSkipBuckets, "audit-skip-buckets", "hermes", "Comma-separated, case-insensitive bucket names excluded from audit (loop prevention for the Hermes audit bucket)")
	opsLogCmd.Flags().StringVar(&opsAuditAllowDomains, "audit-allow-domains", "", "Comma-separated Keystone domains (ID or name) to audit; if set, only these domains are published. Empty = all domains")
	opsLogCmd.Flags().StringVar(&opsAuditDenyDomains, "audit-deny-domains", "", "Comma-separated Keystone domains (ID or name) excluded from audit; takes precedence over --audit-allow-domains")

	// Shortcut flag
	opsLogCmd.Flags().BoolVar(&opsTrackEverything, "track-everything", false, "Enable detailed tracking for all metric types (efficient mode)")
	opsLogCmd.Flags().BoolVar(&opsTrackBucketSLO, "track-bucket-slo", false, "Track low-cardinality bucket GET/LIST SLI metrics for Prometheus SLOs")

	existingOpsLogPreRunE := opsLogCmd.PreRunE
	opsLogCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if opsTrackBucketSLO && !opsPromEnabled {
			return fmt.Errorf("--track-bucket-slo requires --prometheus")
		}
		if existingOpsLogPreRunE != nil {
			return existingOpsLogPreRunE(cmd, args)
		}
		return nil
	}

	// Essential request metrics (most commonly used)
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsDetailed, "track-requests-detailed", false, "Track detailed requests with full labels")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsPerUser, "track-requests-per-user", false, "Track requests aggregated per user")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsPerBucket, "track-requests-per-bucket", false, "Track requests aggregated per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsPerTenant, "track-requests-per-tenant", false, "Track requests aggregated per tenant")

	// Method-based request metrics
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByMethodDetailed, "track-requests-by-method-detailed", false, "Track detailed requests by HTTP method")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByMethodPerUser, "track-requests-by-method-per-user", false, "Track requests by method per user")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByMethodPerBucket, "track-requests-by-method-per-bucket", false, "Track requests by method per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByMethodPerTenant, "track-requests-by-method-per-tenant", false, "Track requests by method per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByMethodGlobal, "track-requests-by-method-global", false, "Track requests by method globally")

	// Operation-based request metrics
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByOperationDetailed, "track-requests-by-operation-detailed", false, "Track detailed requests by operation")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByOperationPerUser, "track-requests-by-operation-per-user", false, "Track requests by operation per user")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByOperationPerBucket, "track-requests-by-operation-per-bucket", false, "Track requests by operation per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByOperationPerTenant, "track-requests-by-operation-per-tenant", false, "Track requests by operation per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByOperationGlobal, "track-requests-by-operation-global", false, "Track requests by operation globally")

	// Status-based request metrics
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByStatusDetailed, "track-requests-by-status-detailed", false, "Track detailed requests by status")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByStatusPerUser, "track-requests-by-status-per-user", false, "Track requests by status per user")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByStatusPerBucket, "track-requests-by-status-per-bucket", false, "Track requests by status per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByStatusPerTenant, "track-requests-by-status-per-tenant", false, "Track requests by status per tenant")

	// Bytes metrics
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentDetailed, "track-bytes-sent-detailed", false, "Track detailed bytes sent")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentPerUser, "track-bytes-sent-per-user", false, "Track bytes sent per user")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentPerBucket, "track-bytes-sent-per-bucket", false, "Track bytes sent per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentPerTenant, "track-bytes-sent-per-tenant", false, "Track bytes sent per tenant")

	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedDetailed, "track-bytes-received-detailed", false, "Track detailed bytes received")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedPerUser, "track-bytes-received-per-user", false, "Track bytes received per user")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedPerBucket, "track-bytes-received-per-bucket", false, "Track bytes received per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedPerTenant, "track-bytes-received-per-tenant", false, "Track bytes received per tenant")

	// Error metrics
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsDetailed, "track-errors-detailed", false, "Track detailed errors")
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsPerUser, "track-errors-per-user", false, "Track errors per user")
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsPerBucket, "track-errors-per-bucket", false, "Track errors per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsPerTenant, "track-errors-per-tenant", false, "Track errors per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsPerStatus, "track-errors-per-status", false, "Track errors per HTTP status")
	opsLogCmd.Flags().BoolVar(&opsTrackTimeoutErrors, "track-timeout-errors", false, "Track timeout errors (408, 504, 598, 499) separately for OSD issues")
	opsLogCmd.Flags().BoolVar(&opsTrackErrorsByCategory, "track-errors-by-category", false, "Track errors by category (timeout, connection, client, server)")

	// IP-based metrics
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByIPDetailed, "track-requests-by-ip-detailed", false, "Track requests by IP")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByIPPerTenant, "track-requests-by-ip-per-tenant", false, "Track requests by IP per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByIPBucketMethodTenant, "track-requests-by-ip-bucket-method-tenant", false, "Track requests by IP, bucket, method and tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackRequestsByIPGlobalPerTenant, "track-requests-by-ip-global-per-tenant", false, "Track requests by IP globally per tenant")

	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentByIPDetailed, "track-bytes-sent-by-ip-detailed", false, "Track bytes sent by IP")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentByIPPerTenant, "track-bytes-sent-by-ip-per-tenant", false, "Track bytes sent by IP per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesSentByIPGlobalPerTenant, "track-bytes-sent-by-ip-global-per-tenant", false, "Track bytes sent by IP globally per tenant")

	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedByIPDetailed, "track-bytes-received-by-ip-detailed", false, "Track bytes received by IP")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedByIPPerTenant, "track-bytes-received-by-ip-per-tenant", false, "Track bytes received by IP per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackBytesReceivedByIPGlobalPerTenant, "track-bytes-received-by-ip-global-per-tenant", false, "Track bytes received by IP globally per tenant")

	opsLogCmd.Flags().BoolVar(&opsTrackErrorsByIP, "track-errors-by-ip", false, "Track errors by IP")

	// Latency metrics
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyDetailed, "track-latency-detailed", false, "Track detailed latency")
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyPerUser, "track-latency-per-user", false, "Track latency per user")
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyPerBucket, "track-latency-per-bucket", false, "Track latency per bucket")
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyPerTenant, "track-latency-per-tenant", false, "Track latency per tenant")
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyPerMethod, "track-latency-per-method", false, "Track latency per method")
	opsLogCmd.Flags().BoolVar(&opsTrackLatencyPerBucketAndMethod, "track-latency-per-bucket-and-method", false, "Track latency per bucket and method")
}

func validateOpsLogConfig(config opslog.OpsLogConfig) {
	missingParams := false

	if config.LogFilePath == "" && config.SocketPath == "" {
		fmt.Println("Warning: --log-file or LOG_FILE_PATH or --socket-path or SOCKET_PATH must be set")
		missingParams = true
	}

	if missingParams {
		fmt.Println("One or more required parameters are missing. Please provide them through flags or environment variables.")
		os.Exit(1)
	}

	// Performance warnings
	if config.MetricsConfig.TrackEverything {
		log.Warn().Msg("Performance Warning: --track-everything enables all metrics. Monitor memory usage in production.")
	}

	// Count enabled detailed metrics (highest memory usage)
	detailedCount := 0
	if config.MetricsConfig.TrackRequestsDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackRequestsByMethodDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackRequestsByOperationDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackRequestsByStatusDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackBytesSentDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackBytesReceivedDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackErrorsDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackRequestsByIPDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackBytesSentByIPDetailed {
		detailedCount++
	}
	if config.MetricsConfig.TrackBytesReceivedByIPDetailed {
		detailedCount++
	}

	if detailedCount > 5 {
		log.Warn().Int("detailed_metrics", detailedCount).Msg("Many detailed metrics enabled - these have highest memory usage")
	}

	// Interval warning for high-frequency environments
	if config.PrometheusIntervalSeconds < 30 && config.MetricsConfig.TrackEverything {
		log.Warn().Int("interval_seconds", config.PrometheusIntervalSeconds).Msg("Short interval with comprehensive tracking may impact performance")
	}
}
