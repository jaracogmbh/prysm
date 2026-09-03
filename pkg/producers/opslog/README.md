# Local Producer - S3 Operations Log

## Overview

The **Local Producer - S3 Operations Log** is a tool designed to process and
monitor S3 operation logs from Ceph RadosGW. It parses log entries, aggregates
metrics, and provides real-time observability by publishing them to NATS or
exposing them in Prometheus format.

## Key Features

- **S3 Log Processing**: Reads and parses Ceph RGW operation logs.
- **NATS Integration**: Publishes raw log events and aggregated metrics to NATS.
- **Prometheus Metrics**: Exposes operation metrics for Prometheus scraping.
- **RabbitMQ Audit Trail**: Publishes CADF-formatted Keystone audit events to
  RabbitMQ for compliance and security monitoring.
- **Latency Tracking**: Real-time request latency histograms with multiple
  aggregation levels.
- **Memory Efficient Architecture**: Dedicated storage maps for each metric
  type ensure minimal memory usage.
- **Log File Rotation Support**: Monitors log file changes and rotates logs
  based on size and retention policies.
- **Configurable**: Allows customization via command-line flags or environment
  variables.
- **Anonymous Request Filtering**: Option to ignore anonymous requests to focus
  on authenticated users.
- **Granular Metrics Control**: Fine-grained toggles to enable/disable specific
  metric categories.
- **Auto Log Rotation on Startup**: Option to rotate log on start to avoid
  reprocessing.
- **Multi-Tenant Support**: Proper tenant separation ensures metrics from
  different tenants are isolated, even for buckets with identical names.
- **Zero-Value Error Metrics**: Error metrics always report 0 when no errors
  occur, ensuring visibility in monitoring dashboards.
- **Timeout Error Detection**: Specialized timeout error tracking (408, 504,
  598, 499) for detecting OSD-related issues.
- **Error Categorization**: Automatic categorization of HTTP errors into
  timeout, connection, client, and server errors.

## Usage

To run the local producer for S3 operations log, use the following command:

```bash
prysm local-producer ops-log [flags]
```

### Example Flags:

- `--log-file "/var/log/ceph/ceph-rgw-ops.json.log"` - Path to the S3
  operations log file.
- `--socket-path "/tmp/ops-log.sock"` - Path to the Unix domain socket.
- `--nats-url "nats://localhost:4222"` - NATS server URL for publishing logs.
- `--nats-subject "rgw.s3.ops"` - NATS subject to publish raw log events.
- `--nats-metrics-subject "rgw.s3.ops.aggregated.metrics"` - NATS subject for
  aggregated metrics.
- `--log-to-stdout` - Enable logging operations to stdout.
- `--log-retention-days 1` - Number of days to retain old log files.
- `--max-log-file-size 10` - Maximum log file size in MB before rotation.
- `--prometheus` - Enable Prometheus metrics.
- `--prometheus-port 8080` - Port for Prometheus metrics.
- `--prometheus-interval 60` - Prometheus metrics update interval in seconds.
- `--ignore-anonymous-requests` - Ignore anonymous requests in metrics.
- `--truncate-log-on-start` - Rotate log on start to avoid re-processing
  existing data.
- `--track-everything` - Enable detailed tracking for all metric types
  (efficient mode).
- `--track-bucket-slo` - Enable low-cardinality bucket GET/LIST SLI metrics for
  Prometheus SLOs.
- `--track-timeout-errors` - Enable tracking of timeout errors (408, 504, 598,
  499) for OSD issue detection.
- `--track-errors-by-category` - Enable error categorization (timeout,
  connection, client, server).
- `--audit-enabled` - Enable RabbitMQ audit trail publishing.
- `--audit-rabbitmq-url` - RabbitMQ connection URL (e.g.,
  `amqp://host:port`; credentials may be embedded or supplied separately).
- `--audit-rabbitmq-username` - RabbitMQ username; overrides any userinfo in
  the URL (e.g. sourced from a Vault entry).
- `--audit-rabbitmq-password` - RabbitMQ password; overrides any userinfo in
  the URL.
- `--audit-queue-name` - RabbitMQ queue name for audit events (default:
  `keystone.notifications.info`). Set to `dataplane.audit` to get a durable
  queue (see note below).
- `--audit-queue-size` - Internal audit event queue size (default: 20).
- `--audit-debug` - Enable debug logging for published audit events.
- `--audit-require-tenant` - Drop audit events that have neither a `project_id`
  nor a `domain_id` (default: true; the customer audit consumer rejects them).
- `--audit-observer-name` - CADF observer name identifying the storage service
  in audit events (default: `radosgw`).
- `--audit-region` - Static region stamped onto each audit event (the ops log
  has none; default: empty = not stamped).
- `--audit-include-reads` - Audit read operations (get_/head_/stat_/list_ prefixed) in addition
  to mutations (default: true, for object-storage data-access auditing). Set
  false for mutations-only.
- `--audit-skip-buckets` - Comma-separated, case-insensitive bucket names
  excluded from audit (default: `hermes`). Breaks the Hermes loop: Hermes writes
  audit events into this bucket, and auditing those writes would re-trigger
  events. Empty disables the filter.
- `--audit-allow-domains` - Comma-separated Keystone domains (matched by domain
  ID *or* name) to audit. When set, only entries whose project domain is in the
  list are published — everything else is dropped (counted as `domain_filtered`).
  Empty = audit all domains. Used to scope the audit trail to specific tenants
  and cut RabbitMQ volume.
- `--audit-deny-domains` - Comma-separated Keystone domains (ID or name) excluded
  from audit. Takes precedence over `--audit-allow-domains`.

### Latency Tracking Examples:

```bash
# Enable all latency tracking
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --prometheus --prometheus-port 8080 \
  --track-latency-detailed \
  --track-latency-per-method \
  --track-latency-per-user \
  --track-latency-per-bucket

# Enable everything with shortcut
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --prometheus --prometheus-port 8080 \
  --track-everything
```

### RabbitMQ Audit Trail Examples:

```bash
# Enable audit trail publishing to RabbitMQ
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --audit-enabled \
  --audit-rabbitmq-url "amqp://guest:guest@rabbitmq.example.com:5672" \
  --audit-queue-name "keystone.notifications.info"

# With debug logging for troubleshooting
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --audit-enabled \
  --audit-rabbitmq-url "amqp://guest:guest@rabbitmq.example.com:5672" \
  --audit-queue-name "keystone.notifications.info" \
  --audit-debug

# Combined monitoring and audit trail
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --prometheus --prometheus-port 8080 \
  --audit-enabled \
  --audit-rabbitmq-url "amqp://guest:guest@rabbitmq.example.com:5672" \
  --audit-queue-name "keystone.notifications.info" \
  --track-everything
```

### Environment Variables

| Environment Variable         | Description                                      |
|------------------------------|--------------------------------------------------|
| `LOG_FILE_PATH`              | Path to the S3 operations log file.             |
| `SOCKET_PATH`                | Path to the Unix domain socket.                 |
| `NATS_URL`                   | NATS server URL.                                |
| `NATS_SUBJECT`               | NATS subject for raw log events.                |
| `NATS_METRICS_SUBJECT`       | NATS subject for aggregated metrics.            |
| `LOG_TO_STDOUT`              | Enable logging operations to stdout.            |
| `LOG_RETENTION_DAYS`         | Number of days to retain old log files.         |
| `MAX_LOG_FILE_SIZE`          | Maximum log file size before rotation (in MB).  |
| `PROMETHEUS_PORT`            | Port for Prometheus metrics.                    |
| `PROMETHEUS_INTERVAL`        | Prometheus metrics update interval in seconds.  |
| `IGNORE_ANONYMOUS_REQUESTS`  | Ignore anonymous requests in metrics.           |
| `TRUNCATE_LOG_ON_START`      | Whether to rotate the log file on startup.      |
| `TRACK_EVERYTHING`           | Enable detailed tracking for all metric types.  |
| `TRACK_BUCKET_SLO`           | Enable low-cardinality bucket GET/LIST SLI metrics. |
| `AUDIT_ENABLED`              | Enable RabbitMQ audit trail publishing.         |
| `AUDIT_RABBITMQ_URL`         | RabbitMQ connection URL.                        |
| `AUDIT_RABBITMQ_USERNAME`    | RabbitMQ username; overrides URL userinfo.      |
| `AUDIT_RABBITMQ_PASSWORD`    | RabbitMQ password; overrides URL userinfo.      |
| `AUDIT_QUEUE_NAME`           | RabbitMQ queue name (`dataplane.audit` = durable). |
| `AUDIT_QUEUE_SIZE`           | Internal audit event queue size.                |
| `AUDIT_DEBUG`                | Enable debug logging for audit events.          |
| `AUDIT_REQUIRE_TENANT`       | Drop events without a project_id/domain_id (default true). |
| `AUDIT_OBSERVER_NAME`        | CADF observer (storage service) name (default radosgw). |
| `AUDIT_REGION`               | Static region stamped on events (empty = off).  |
| `AUDIT_INCLUDE_READS`        | Audit reads (get_/head_/stat_/list_ prefixed) too (default true). |
| `AUDIT_SKIP_BUCKETS`         | Buckets excluded from audit, comma-list (default `hermes`). |
| `AUDIT_ALLOW_DOMAINS`        | Keystone domains (ID or name, comma-list) to audit; only these are published when set. |
| `AUDIT_DENY_DOMAINS`         | Keystone domains (ID or name, comma-list) excluded; precedes `AUDIT_ALLOW_DOMAINS`. |

#### Request Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_REQUESTS_DETAILED`                     | Track detailed requests with full labels.                     |
| `TRACK_REQUESTS_PER_USER`                     | Track requests aggregated per user.                           |
| `TRACK_REQUESTS_PER_BUCKET`                   | Track requests aggregated per bucket.                         |
| `TRACK_REQUESTS_PER_TENANT`                   | Track requests aggregated per tenant.                         |

#### Method-based Request Tracking:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_REQUESTS_BY_METHOD_DETAILED`           | Track detailed requests by HTTP method.                       |
| `TRACK_REQUESTS_BY_METHOD_PER_USER`           | Track requests by method per user.                            |
| `TRACK_REQUESTS_BY_METHOD_PER_BUCKET`         | Track requests by method per bucket.                          |
| `TRACK_REQUESTS_BY_METHOD_PER_TENANT`         | Track requests by method per tenant.                          |
| `TRACK_REQUESTS_BY_METHOD_GLOBAL`             | Track requests by method globally.                            |

#### Operation-based Request Tracking:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_REQUESTS_BY_OPERATION_DETAILED`        | Track detailed requests by operation.                         |
| `TRACK_REQUESTS_BY_OPERATION_PER_USER`        | Track requests by operation per user.                         |
| `TRACK_REQUESTS_BY_OPERATION_PER_BUCKET`      | Track requests by operation per bucket.                       |
| `TRACK_REQUESTS_BY_OPERATION_PER_TENANT`      | Track requests by operation per tenant.                       |
| `TRACK_REQUESTS_BY_OPERATION_GLOBAL`          | Track requests by operation globally.                         |

#### Status-based Request Tracking:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_REQUESTS_BY_STATUS_DETAILED`           | Track detailed requests by status.                            |
| `TRACK_REQUESTS_BY_STATUS_PER_USER`           | Track requests by status per user.                            |
| `TRACK_REQUESTS_BY_STATUS_PER_BUCKET`         | Track requests by status per bucket.                          |
| `TRACK_REQUESTS_BY_STATUS_PER_TENANT`         | Track requests by status per tenant.                          |

#### Bytes Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_BYTES_SENT_DETAILED`                   | Track detailed bytes sent.                                    |
| `TRACK_BYTES_SENT_PER_USER`                   | Track bytes sent per user.                                    |
| `TRACK_BYTES_SENT_PER_BUCKET`                 | Track bytes sent per bucket (with tenant separation).         |
| `TRACK_BYTES_SENT_PER_TENANT`                 | Track bytes sent per tenant.                                  |
| `TRACK_BYTES_RECEIVED_DETAILED`               | Track detailed bytes received.                                |
| `TRACK_BYTES_RECEIVED_PER_USER`               | Track bytes received per user.                                |
| `TRACK_BYTES_RECEIVED_PER_BUCKET`             | Track bytes received per bucket (with tenant separation).     |
| `TRACK_BYTES_RECEIVED_PER_TENANT`             | Track bytes received per tenant.                              |

#### Error Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_ERRORS_DETAILED`                       | Track detailed errors.                                        |
| `TRACK_ERRORS_PER_USER`                       | Track errors per user.                                        |
| `TRACK_ERRORS_PER_BUCKET`                     | Track errors per bucket (with tenant separation).             |
| `TRACK_ERRORS_PER_TENANT`                     | Track errors per tenant.                                      |
| `TRACK_ERRORS_PER_STATUS`                     | Track errors per HTTP status code.                            |
| `TRACK_ERRORS_BY_IP`                          | Track errors by IP address.                                   |
| `TRACK_TIMEOUT_ERRORS`                        | Track timeout errors (408, 504, 598, 499) for OSD detection.  |
| `TRACK_ERRORS_BY_CATEGORY`                    | Track errors by category (timeout, connection, client, server).|

#### IP-based Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_REQUESTS_BY_IP_DETAILED`               | Track requests by IP.                                         |
| `TRACK_REQUESTS_BY_IP_PER_TENANT`             | Track requests by IP per tenant.                              |
| `TRACK_REQUESTS_BY_IP_BUCKET_METHOD_TENANT`   | Track requests by IP, bucket, method and tenant.              |
| `TRACK_REQUESTS_BY_IP_GLOBAL_PER_TENANT`      | Track requests by IP globally per tenant.                     |
| `TRACK_BYTES_SENT_BY_IP_DETAILED`             | Track bytes sent by IP.                                       |
| `TRACK_BYTES_SENT_BY_IP_PER_TENANT`           | Track bytes sent by IP per tenant.                            |
| `TRACK_BYTES_SENT_BY_IP_GLOBAL_PER_TENANT`    | Track bytes sent by IP globally per tenant.                   |
| `TRACK_BYTES_RECEIVED_BY_IP_DETAILED`         | Track bytes received by IP.                                   |
| `TRACK_BYTES_RECEIVED_BY_IP_PER_TENANT`       | Track bytes received by IP per tenant.                        |
| `TRACK_BYTES_RECEIVED_BY_IP_GLOBAL_PER_TENANT`| Track bytes received by IP globally per tenant.               |

#### Latency Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_LATENCY_DETAILED`                      | Track detailed latency with full labels.                      |
| `TRACK_LATENCY_PER_USER`                      | Track latency aggregated per user.                            |
| `TRACK_LATENCY_PER_BUCKET`                    | Track latency aggregated per bucket.                          |
| `TRACK_LATENCY_PER_TENANT`                    | Track latency aggregated per tenant.                          |
| `TRACK_LATENCY_PER_METHOD`                    | Track latency aggregated per HTTP method.                     |
| `TRACK_LATENCY_PER_BUCKET_AND_METHOD`         | Track latency by bucket and method combination.               |

#### SLI Tracking Environment Variables:

| Variable                                      | Description                                                    |
|-----------------------------------------------|----------------------------------------------------------------|
| `TRACK_BUCKET_SLO`                            | Track low-cardinality bucket GET/LIST request SLI metrics for Prometheus SLOs. |

## Metrics Collected

### Request Counters

| Metric Name                           | Type      | Labels                                               | Description                                                        |
|---------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_total_requests`              | Counter   | `pod`, `user`, `tenant`, `bucket`, `method`, `http_status` | Total number of requests processed with full dimensionality.     |
| `radosgw_total_requests_per_user`     | Counter   | `pod`, `user`, `tenant`, `method`, `http_status`     | Total requests aggregated per user (all buckets combined).        |
| `radosgw_total_requests_per_bucket`   | Counter   | `pod`, `tenant`, `bucket`, `method`, `http_status`   | Total requests aggregated per bucket (all users combined).        |
| `radosgw_total_requests_per_tenant`   | Counter   | `pod`, `tenant`, `method`, `http_status`             | Total requests aggregated per tenant (all users and buckets).     |

### Method-based Request Counters

| Metric Name                                   | Type      | Labels                                               | Description                                                        |
|-----------------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_requests_by_method`                  | Counter   | `pod`, `user`, `tenant`, `bucket`, `method`          | Number of requests grouped by HTTP method with full detail.       |
| `radosgw_requests_by_method_per_user`         | Counter   | `pod`, `user`, `tenant`, `method`                    | Number of requests by method aggregated per user.                 |
| `radosgw_requests_by_method_per_bucket`       | Counter   | `pod`, `tenant`, `bucket`, `method`                  | Number of requests by method aggregated per bucket.               |
| `radosgw_requests_by_method_per_tenant`       | Counter   | `pod`, `tenant`, `method`                            | Number of requests by method aggregated per tenant.               |
| `radosgw_requests_by_method_global`           | Counter   | `pod`, `method`                                      | Number of requests by method globally aggregated.                 |

### Operation-based Request Counters

| Metric Name                                   | Type      | Labels                                               | Description                                                        |
|-----------------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_requests_by_operation`               | Counter   | `pod`, `user`, `tenant`, `bucket`, `operation`, `method` | Number of requests grouped by operation with full detail.         |
| `radosgw_requests_by_operation_per_user`      | Counter   | `pod`, `user`, `tenant`, `operation`, `method`       | Number of requests by operation aggregated per user.              |
| `radosgw_requests_by_operation_per_bucket`    | Counter   | `pod`, `tenant`, `bucket`, `operation`, `method`     | Number of requests by operation aggregated per bucket.            |
| `radosgw_requests_by_operation_per_tenant`    | Counter   | `pod`, `tenant`, `operation`, `method`               | Number of requests by operation aggregated per tenant.            |
| `radosgw_requests_by_operation_global`        | Counter   | `pod`, `operation`, `method`                         | Number of requests by operation globally aggregated.              |

### Status-based Request Counters

| Metric Name                                   | Type      | Labels                                               | Description                                                        |
|-----------------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_requests_by_status_detailed`         | Counter   | `pod`, `user`, `tenant`, `bucket`, `status`          | Number of requests grouped by HTTP status with full detail.       |
| `radosgw_requests_by_status_per_user`         | Counter   | `pod`, `user`, `tenant`, `status`                    | Number of requests by status aggregated per user.                 |
| `radosgw_requests_by_status_per_bucket`       | Counter   | `pod`, `tenant`, `bucket`, `status`                  | Number of requests by status aggregated per bucket.               |
| `radosgw_requests_by_status_per_tenant`       | Counter   | `pod`, `tenant`, `status`                            | Number of requests by status aggregated per tenant.               |

### Bytes Transferred Counters

| Metric Name                           | Type      | Labels                                               | Description                                                        |
|---------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_bytes_sent`                  | Counter   | `pod`, `user`, `tenant`, `bucket`                    | Total number of bytes sent with proper tenant separation.         |
| `radosgw_bytes_received`              | Counter   | `pod`, `user`, `tenant`, `bucket`                    | Total number of bytes received with proper tenant separation.     |
| `radosgw_bytes_sent_per_user`         | Counter   | `pod`, `user`, `tenant`                              | Total bytes sent aggregated per user (all buckets combined).      |
| `radosgw_bytes_received_per_user`     | Counter   | `pod`, `user`, `tenant`                              | Total bytes received aggregated per user (all buckets combined).  |
| `radosgw_bytes_sent_per_bucket`       | Counter   | `pod`, `tenant`, `bucket`                            | Total bytes sent aggregated per bucket (all users combined).      |
| `radosgw_bytes_received_per_bucket`   | Counter   | `pod`, `tenant`, `bucket`                            | Total bytes received aggregated per bucket (all users combined).  |
| `radosgw_bytes_sent_per_tenant`       | Counter   | `pod`, `tenant`                                      | Total bytes sent aggregated per tenant (all users and buckets).   |
| `radosgw_bytes_received_per_tenant`   | Counter   | `pod`, `tenant`                                      | Total bytes received aggregated per tenant (all users and buckets). |

### Error Counters

| Metric Name                           | Type      | Labels                                               | Description                                                        |
|---------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_errors_detailed`             | Counter   | `pod`, `user`, `tenant`, `bucket`, `http_status`     | Total number of errors with full detail. **Always shows 0 when no errors**.  |
| `radosgw_errors_per_user`             | Counter   | `pod`, `user`, `tenant`, `http_status`               | Total errors aggregated per user. **Always visible with value 0 when no errors**. |
| `radosgw_errors_per_bucket`           | Counter   | `pod`, `tenant`, `bucket`, `http_status`             | Total errors aggregated per bucket. **Always visible with value 0 when no errors**. |
| `radosgw_errors_per_tenant`           | Counter   | `pod`, `tenant`, `http_status`                       | Total errors aggregated per tenant. **Always visible with value 0 when no errors**. |
| `radosgw_errors_per_status`           | Counter   | `pod`, `http_status`                                 | Total errors aggregated per HTTP status code. **Always visible with value 0 when no errors**. |
| `radosgw_errors_per_ip`               | Counter   | `pod`, `ip`, `tenant`, `http_status`                 | Total errors aggregated per IP address. **Always visible with value 0 when no errors**. |

### Timeout Error Counters (New)

| Metric Name                           | Type      | Labels                                               | Description                                                        |
|---------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_timeout_errors`              | Counter   | `pod`, `user`, `tenant`, `bucket`, `timeout_type`    | Total timeout errors by type (408, 504, 598, 499) for OSD issue detection. |

### Error Category Counters (New)

| Metric Name                           | Type      | Labels                                               | Description                                                        |
|---------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_errors_by_category`          | Counter   | `pod`, `user`, `tenant`, `bucket`, `category`        | Errors categorized as: timeout, connection, client, server for better monitoring. |

### IP-based Gauges

| Metric Name                                  | Type      | Labels                                               | Description                                                        |
|----------------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_requests_by_ip`                     | Gauge     | `pod`, `user`, `tenant`, `ip`                        | Total number of requests grouped by IP and user.                  |
| `radosgw_requests_per_ip`                    | Gauge     | `pod`, `tenant`, `ip`                                | Total requests aggregated per IP (all users combined).            |
| `radosgw_requests_per_tenant_from_ip`        | Gauge     | `pod`, `tenant`                                      | Total requests aggregated per tenant from all IPs.                |
| `radosgw_requests_by_ip_bucket_method_tenant`| Gauge     | `pod`, `ip`, `bucket`, `method`, `tenant`            | Total number of requests grouped by IP, bucket and method.        |
| `radosgw_bytes_sent_by_ip`                   | Gauge     | `pod`, `user`, `tenant`, `ip`                        | Total bytes sent grouped by IP and user.                          |
| `radosgw_bytes_sent_per_ip`                  | Gauge     | `pod`, `tenant`, `ip`                                | Total bytes sent aggregated per IP (all users combined).          |
| `radosgw_bytes_sent_per_tenant_from_ip`      | Gauge     | `pod`, `tenant`                                      | Total bytes sent aggregated per tenant from all IPs.              |
| `radosgw_bytes_received_by_ip`               | Gauge     | `pod`, `user`, `tenant`, `ip`                        | Total bytes received grouped by IP and user.                      |
| `radosgw_bytes_received_per_ip`              | Gauge     | `pod`, `tenant`, `ip`                                | Total bytes received aggregated per IP (all users combined).      |
| `radosgw_bytes_received_per_tenant_from_ip`  | Gauge     | `pod`, `tenant`                                      | Total bytes received aggregated per tenant from all IPs.          |

### Latency Histograms

| Metric Name                                          | Type      | Labels                                               | Description                                                        |
|------------------------------------------------------|-----------|------------------------------------------------------|--------------------------------------------------------------------|
| `radosgw_requests_duration`                          | Histogram | `user`, `tenant`, `bucket`, `method`                 | Histogram of request latencies with full detail (in seconds).     |
| `radosgw_requests_duration_per_user`                 | Histogram | `user`, `tenant`, `method`                           | Histogram for request latencies aggregated per user (all buckets combined). |
| `radosgw_requests_duration_per_bucket`               | Histogram | `tenant`, `bucket`, `method`                         | Histogram for request latencies aggregated per bucket (all users combined). |
| `radosgw_requests_duration_per_tenant`               | Histogram | `tenant`, `method`                                   | Histogram for request latencies aggregated per tenant (all users and buckets combined). |
| `radosgw_requests_duration_per_method`               | Histogram | `method`                                             | Histogram for request latencies aggregated per method (global).   |
| `radosgw_requests_duration_per_bucket_and_method`    | Histogram | `tenant`, `bucket`, `method`                         | Histogram for request latencies aggregated per bucket and method (all users combined). |

### Bucket SLI Metrics

| Metric Name                                   | Type      | Labels                                      | Description                                                        |
|-----------------------------------------------|-----------|---------------------------------------------|--------------------------------------------------------------------|
| `radosgw_bucket_sli_requests_total`           | Counter   | `tenant`, `bucket`, `operation`, `status_class` | Low-cardinality bucket SLI request counter for GET/LIST-style operations, labeled by response class such as `2xx` or `5xx`. |
| `radosgw_bucket_sli_request_duration_seconds` | Histogram | `tenant`, `bucket`, `operation`             | Latency histogram in seconds for bucket GET/LIST SLI operations, intended for Prometheus SLO evaluation. |

> **Note**: Histogram metrics do **not** include the `pod` label to reduce
> cardinality. Each histogram automatically provides `_bucket`, `_count`, and
> `_sum` metrics for comprehensive latency analysis.

### Memory Efficiency Architecture

The system uses a **dedicated storage architecture** where each metric type has
its own optimized storage map:

- **Memory Efficient**: Only enabled metric types consume memory
- **Optimal Granularity**: Each aggregation level stores exactly the data it
  needs
- **No Runtime Aggregation**: All aggregation happens at storage time for
  better performance
- **Independent Metrics**: Each metric can be enabled/disabled independently
  without affecting others

### Multi-Tenant Support

All bucket-level metrics now properly separate tenants to avoid data collision:
- **Bucket metrics** include tenant information to distinguish between buckets
  with identical names across different tenants
- **IP-based error tracking** includes tenant context for proper attribution
- **Aggregation levels** provide both tenant-specific and tenant-aggregated
  views for flexible monitoring

### Metric Aggregation Levels

Many metrics provide multiple aggregation levels for flexible monitoring:
- **Full granularity**: Complete dimensional breakdown (user, tenant, bucket,
  method, status)
- **Per-user**: Aggregated by user across all buckets
- **Per-bucket**: Aggregated by bucket with tenant separation across all users
- **Per-tenant**: Aggregated across all buckets and users within a tenant
- **Global**: Fully aggregated across all dimensions

## RabbitMQ Audit Trail

The ops-log producer can publish CADF-formatted (Cloud Auditing Data
Federation) audit events to RabbitMQ for compliance and security monitoring.
These events are consumed by Hermes and other audit processing systems.

### Features

- **CADF Format Compliance**: All audit events follow the CADF 1.0 standard
- **Keystone Integration**: Includes full Keystone scope information (project,
  user, domain, roles)
- **Application Credentials**: Properly tracks API calls made with application
  credentials
- **Fire-and-Forget Publishing**: Non-blocking audit event publishing that
  doesn't impact log processing
- **Graceful Degradation**: Uses NullAuditor when RabbitMQ is unavailable
  (development/testing)
- **Buffered Channel**: 20-event internal queue with automatic retry on
  failures
- **Tenant Enforcement**: With `AUDIT_REQUIRE_TENANT=true` (default), events
  without a `project_id` or `domain_id` are dropped before publishing — the
  customer audit consumer (Hermes) requires a tenant. Dropped events are
  counted in `prysm_audit_events_dropped_total{reason="no_tenant"}`, not
  silently discarded.

### Event filtering

Audit emission is gated before publishing. Each drop is counted (not silent) in
`prysm_audit_events_dropped_total{reason}`:

- **`skip_bucket`** — operations on a bucket listed in `AUDIT_SKIP_BUCKETS`
  (default `hermes`) are excluded. This breaks the Hermes loop: Hermes writes
  audit events into a per-customer (WORM) bucket, and auditing those writes
  would re-trigger events indefinitely. Bucket-name matching is correct here
  because the audit bucket lives inside each customer's project — only Hermes
  can write to it (WORM), so nothing legitimate is lost.
- **`domain_filtered`** — the entry's Keystone project domain is excluded by the
  domain scope: it is listed in `AUDIT_DENY_DOMAINS`, or `AUDIT_ALLOW_DOMAINS` is
  set and the domain is not in it. Domains match by ID or name. Use this to scope
  the audit trail to specific tenants and reduce RabbitMQ volume.
- **`no_tenant`** — events without a `project_id`/`domain_id` (see Tenant
  Enforcement above).
- **`read`** — read operations (get_/head_/stat_/list_ prefixed) when `AUDIT_INCLUDE_READS=false`.
  Reads are audited by default (object-storage data-access events); set the flag
  false for a mutations-only trail.

### Durable queue

The underlying `go-bits/audittools` library declares the RabbitMQ queue
**durable** only when `AUDIT_QUEUE_NAME` is exactly `dataplane.audit`; any other
name is transient. The dataplane audit log-router consumes `dataplane.audit` and
requires a durable queue, so set `AUDIT_QUEUE_NAME=dataplane.audit` for it to
connect. Note: the queue survives a broker restart, but messages are still
published transient. If the queue already exists with a different durability
flag, delete it first — RabbitMQ rejects a redeclare with `406
PRECONDITION_FAILED`.

### Observer and region

- **Observer**: events identify the storage service via
  `observer = { typeURI: "service/storage", name: <AUDIT_OBSERVER_NAME> }`
  (default name `radosgw`).
- **Region**: the ops log carries no region, so it can be supplied statically
  via `AUDIT_REGION` and is stamped onto the target as a `region` attachment.
  Leave empty to omit. (Placement may change pending audit-consumer
  confirmation.)

### CADF Event Structure

Each audit event includes:

- **Initiator**: Keystone user information with project, domain, roles, and
  application credential details
- **Target**: Object (with bucket attachment), Bucket, or Account target types
- **Action**: Mapped RadosGW operations (`read`, `read/list`, `create`,
  `delete`, `update`, `update/copy`)
- **Observer**: Prysm ops-log service identification
- **Outcome**: Success/failure with HTTP status code
- **Request Path**: Full Swift/S3 URI path

### Example CADF Event

```json
{
  "typeURI": "http://schemas.dmtf.org/cloud/audit/1.0/event",
  "eventTime": "2025-11-05T09:38:35.042215+00:00",
  "action": "read",
  "outcome": "success",
  "reason": {
    "reasonType": "HTTP",
    "reasonCode": "200"
  },
  "initiator": {
    "typeURI": "service/security/account/user",
    "name": "charlie",
    "id": "9e83cf791297602fc0f40496726e46a0",
    "project_id": "1a234ef54d4f6f930a5d03cfa9e186f6",
    "project_name": "dev",
    "domain_id": "default",
    "domain_name": "Default",
    "application_credential_id": "cad3f50c10ff628a1d7dc1694ffc1c62",
    "attachments": [{
      "name": "application_credential_name",
      "content": "dev-app-cred-charlie"
    }]
  },
  "target": {
    "typeURI": "service/storage/object",
    "name": "data/report.pdf",
    "id": "backup-bucket/data/report.pdf",
    "attachments": [{
      "name": "bucket",
      "typeURI": "service/storage/bucket",
      "content": "backup-bucket"
    }]
  },
  "observer": {
    "typeURI": "service/storage/object",
    "name": "prysm-ops-log"
  }
}
```

### Operation to Action Mapping

| RadosGW Operation | CADF Action    | Description                    |
|-------------------|----------------|--------------------------------|
| `list_buckets`    | `read/list`    | List all buckets              |
| `list_bucket`     | `read/list`    | List objects in bucket        |
| `get_obj`         | `read`         | Download object (also HEAD on object) |
| `get_bucket_info` | `read`         | Get bucket metadata (admin API) |
| `stat_bucket`     | `read`         | HEAD bucket                   |
| `stat_account`    | `read`         | HEAD account (Swift)          |
| `put_obj`         | `create`       | Upload new object             |
| `post_obj`        | `update`       | Swift form POST / S3 browser upload |
| `create_bucket`   | `create`       | Create new bucket             |
| `bulk_upload`     | `create`       | Swift bulk upload (tar)       |
| `restore_obj`     | `create`       | S3 RestoreObject              |
| `delete_obj`      | `delete`       | Delete object                 |
| `delete_bucket`   | `delete`       | Delete bucket                 |
| `multi_object_delete` | `delete`   | S3 multi-object delete        |
| `bulk_delete`     | `delete`       | Swift bulk delete             |
| `copy_obj`        | `update/copy`  | Copy object                   |

### Configuration

Enable audit trail with these flags:

```bash
--audit-enabled                    # Enable audit event publishing
--audit-rabbitmq-url               # RabbitMQ connection (amqp://user:pass@host:port)
--audit-queue-name                 # Queue name (default: keystone.notifications.info)
--audit-queue-size                 # Internal queue size (default: 20)
--audit-debug                      # Log published events for troubleshooting
```

### Behavior

- **RabbitMQ Available**: Audit events published successfully
- **RabbitMQ Unavailable at Startup**: Falls back to NullAuditor (no-op), logs
  warning
- **RabbitMQ Connection Lost During Runtime**: Internal queue buffers events,
  automatic retry
- **Development/Testing**: Use NullAuditor by omitting `--audit-enabled` or
  `--audit-rabbitmq-url`

### Notes

- Audit events are published asynchronously and never block ops log processing
- File watcher (fsnotify) only triggers on NEW writes to the log file
- Ops log is truncated on prysm startup (ephemeral sidecar architecture)
- Full Keystone scope is required in ops log entries for proper audit tracking

## Workflow

1. **Log Processing**: Reads and parses log entries incoming from the Ceph RGW
   log file.
2. **Dedicated Storage**: Updates dedicated storage maps based on enabled
   metric types with proper tenant separation.
3. **Latency Recording**: Records request latencies from the `total_time` field
   directly into Prometheus histograms.
4. **Audit Trail Publishing** *(optional)*: Converts ops log entries to CADF
   format and publishes to RabbitMQ asynchronously.
5. **Publishing to NATS**: Raw log events and aggregated metrics are sent to
   specified NATS subjects.
6. **Prometheus Metrics**: Exposes metrics via an HTTP server for Prometheus
   scraping.
7. **File Rotation Handling**: Monitors log file size and age, triggering
   rotation when needed.
8. **Log Rotation on Start** *(optional)*: Backs up and clears the log file at
   startup to avoid re-processing.

## Example Workflows

### Basic Monitoring with Latency Tracking

```bash
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --prometheus --prometheus-port 8080 \
  --track-latency-detailed \
  --track-latency-per-method \
  --truncate-log-on-start
```

### Comprehensive Monitoring

```bash
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --nats-url nats://localhost:4222 \
  --prometheus --prometheus-port 8080 \
  --prometheus-interval 30 \
  --track-everything \
  --ignore-anonymous-requests
```

### Minimal Resource Usage

```bash
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --prometheus --prometheus-port 8080 \
  --track-latency-per-method \
  --track-requests-per-tenant \
  --track-errors-per-user
```

### Audit Trail for Compliance

```bash
# Production audit trail with monitoring
prysm local-producer ops-log \
  --log-file /var/log/ceph/ops-log.log \
  --audit-enabled \
  --audit-rabbitmq-url "amqp://audit-user:password@rabbitmq.prod.example.com:5672" \
  --audit-queue-name "keystone.notifications.info" \
  --prometheus --prometheus-port 8080 \
  --track-latency-per-method \
  --track-requests-per-tenant
```

## Configuration Best Practices

### Performance Considerations

- **Use `--track-everything` carefully**: While convenient, it creates many
  metrics which can impact performance
- **Selective tracking**: Enable only the metrics you actually need for
  monitoring
- **Latency tracking**: Start with `--track-latency-per-method` and add more
  granular tracking as needed
- **Anonymous requests**: Use `--ignore-anonymous-requests` to reduce noise in
  multi-tenant environments
- **Memory efficiency**: Each metric type uses dedicated storage, so only
  enabled metrics consume memory

### Recommended Configurations

**For development/testing:**
```bash
--track-everything --prometheus-interval 10
```

**For production (minimal):**
```bash
--track-latency-per-method --track-requests-per-tenant --track-errors-per-user
```

**For production (comprehensive):**
```bash
--track-latency-detailed --track-latency-per-method --track-requests-per-user --track-requests-per-bucket --track-errors-per-user --track-bytes-sent-per-bucket
```

## Error Monitoring Best Practices

### Zero-Value Error Metrics
All error metrics now report 0 when no errors occur, ensuring they remain
visible in Prometheus and Grafana dashboards. This improvement:
- Eliminates the "No data" issue in dashboards
- Allows for proper rate calculations even when errors are intermittent
- Ensures alerting rules work correctly with absent metrics

### Timeout Error Detection for OSD Issues
The new `radosgw_timeout_errors` metric specifically tracks timeout-related
HTTP status codes:
- **408 (Request Timeout)**: Client took too long to send request
- **504 (Gateway Timeout)**: Upstream server timeout (often indicates OSD
  issues)
- **598 (Network Read Timeout)**: Network-level timeout
- **499 (Client Closed Request)**: Client disconnected before response

Use these metrics to detect OSD performance issues:
```promql
# Alert when timeout errors exceed threshold
rate(radosgw_timeout_errors[5m]) > 0.1
```

### Error Categorization
The `radosgw_errors_by_category` metric automatically categorizes errors:
- **timeout**: 408, 504, 598, 499 status codes
- **connection**: 502, 503 status codes
- **client**: 4xx errors (excluding timeouts)
- **server**: 5xx errors (excluding timeouts and connection errors)

This simplifies monitoring and alerting:
```promql
# Alert on server errors
rate(radosgw_errors_by_category{category="server"}[5m]) > 0.05

# Alert on connection issues
rate(radosgw_errors_by_category{category="connection"}[5m]) > 0.1
```

## Notes

- Ensure that the Ceph RGW log format is JSON-based to be compatible with this
  tool.
- If using NATS, ensure the server is running and accessible from the producer.
- If using RabbitMQ audit trail, ensure the RabbitMQ server is accessible and
  the queue exists.
- Prometheus should be configured to scrape the exposed metrics endpoint.
- **Multi-tenant environments**: The tool automatically extracts tenant
  information from user identifiers and ensures proper separation of metrics
  across tenants.
- **Bucket name collision handling**: Buckets with identical names from
  different tenants are properly isolated in all metrics.
- **Latency units**: All latency histograms use seconds as the unit, converted
  from the millisecond `total_time` field in log entries.
- **Memory efficiency**: The dedicated storage architecture ensures minimal
  memory usage by storing only enabled metric types.
- **Error visibility**: Error metrics always maintain visibility by reporting 0
  when no errors occur, essential for proper monitoring.
- **Audit trail**: CADF events are published asynchronously and never block ops
  log processing. Falls back to NullAuditor if RabbitMQ is unavailable.
- **Keystone scope**: Full Keystone authentication scope (including application
  credentials) is required in ops log entries for complete audit tracking.
- Sidecar injection is supported via a mutating webhook (see related
  documentation for Kubernetes usage).

> This README will be updated as new features and improvements are introduced.
> Contributions and feedback are welcome!
