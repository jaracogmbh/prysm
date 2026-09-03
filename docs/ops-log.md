# Ops Log producer

Parses RadosGW operation logs and exposes Prometheus metrics. A Kubernetes mutating webhook injects it as a sidecar into RGW pods.

Can also publish CADF audit events to RabbitMQ for compliance and security monitoring.

## Prerequisites

- Rook-Ceph operator with a CephObjectStore deployed
- cert-manager installed
- RabbitMQ reachable from the cluster (only if you want the audit trail)

## Deployment

### Step 1: Create the webhook namespace

```bash
kubectl create namespace webhook
```

### Step 2: cert-manager resources

The webhook needs TLS. cert-manager handles certificate generation.

```bash
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: webhook
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: prysm-webhook-cert
  namespace: webhook
spec:
  secretName: prysm-webhook-cert
  dnsNames:
    - prysm-webhook-service.webhook.svc
  issuerRef:
    name: selfsigned-issuer
    kind: Issuer
EOF
```

### Step 3: Webhook server

```bash
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prysm-webhook-service
  namespace: webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prysm-webhook-service
  template:
    metadata:
      labels:
        app: prysm-webhook-service
    spec:
      containers:
        - name: prysmwebhook
          image: ghcr.io/cobaltcore-dev/prysm-webhook:0.0.36
          ports:
            - containerPort: 8443
          volumeMounts:
            - name: certs
              mountPath: /certs
              readOnly: true
          env:
            - name: SIDECAR_IMAGE
              value: "ghcr.io/cobaltcore-dev/prysm:0.0.36"
          imagePullPolicy: Always
      volumes:
        - name: certs
          secret:
            secretName: prysm-webhook-cert
EOF
```

Pin `SIDECAR_IMAGE` to a specific version tag. This is the prysm image that gets injected into RGW pods.

### Step 4: Webhook service

```bash
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: prysm-webhook-service
  namespace: webhook
spec:
  selector:
    app: prysm-webhook-service
  ports:
    - port: 443
      targetPort: 8443
EOF
```

### Step 5: Register the mutating webhook

```bash
kubectl apply -f - <<'EOF'
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: prysm-webhook
  annotations:
    cert-manager.io/inject-ca-from: "webhook/prysm-webhook-cert"
webhooks:
  - name: prysm-webhook.injector.webhook
    clientConfig:
      service:
        name: prysm-webhook-service
        namespace: webhook
        path: "/mutate"
    admissionReviewVersions: ["v1"]
    sideEffects: None
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["apps"]
        apiVersions: ["v1"]
        resources: ["deployments"]
EOF
```

### Step 6: Enable sidecar injection

Add `prysm-sidecar: "yes"` to your CephObjectStore gateway labels:

```yaml
apiVersion: ceph.rook.io/v1
kind: CephObjectStore
metadata:
  name: my-store
  namespace: rook-ceph
spec:
  gateway:
    labels:
      prysm-sidecar: "yes"
```

The webhook injects the sidecar only when all five labels are present:

| Label | Value |
|-------|-------|
| `app` | `rook-ceph-rgw` |
| `app.kubernetes.io/component` | `cephobjectstores.ceph.rook.io` |
| `app.kubernetes.io/created-by` | `rook-ceph-operator` |
| `app.kubernetes.io/managed-by` | `rook-ceph-operator` |
| `prysm-sidecar` | `yes` |

Rook sets the first four automatically. You only add `prysm-sidecar: "yes"`.

### Step 7: Configure the sidecar

The injected sidecar starts with these defaults:

```
local-producer ops-log --log-file=/var/log/ceph/ops-log.log --max-log-file-size=10 --prometheus=true --prometheus-port=9090 -v=info
```

To override behavior, create a Secret or ConfigMap and reference it via annotations.

#### Option A: Secret (for credentials)

```bash
kubectl -n rook-ceph create secret generic prysm-sidecar-env \
  --from-literal=AUDIT_ENABLED="true" \
  --from-literal=AUDIT_RABBITMQ_URL="amqp://user:password@rabbitmq.rook-ceph.svc:5672" \
  --from-literal=AUDIT_QUEUE_NAME="keystone.notifications.info"
```

Add the annotation to your CephObjectStore:

```yaml
spec:
  gateway:
    labels:
      prysm-sidecar: "yes"
    annotations:
      prysm-sidecar/sidecar-env-secret: "prysm-sidecar-env"
```

#### Option B: ConfigMap (for non-sensitive config)

```bash
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: prysm-sidecar-config
  namespace: rook-ceph
data:
  LOG_FILE_PATH: "/var/log/ceph/ops-log.log"
  MAX_LOG_FILE_SIZE: "10"
  PROMETHEUS_PORT: "9090"
  IGNORE_ANONYMOUS_REQUESTS: "true"
  TRACK_REQUESTS_PER_TENANT: "true"
  TRACK_LATENCY_PER_METHOD: "true"
  TRACK_ERRORS_PER_USER: "true"
  TRACK_BYTES_SENT_PER_BUCKET: "true"
  TRACK_BYTES_RECEIVED_PER_BUCKET: "true"
EOF
```

Add the annotation:

```yaml
annotations:
  prysm-sidecar/sidecar-env-configmap: "prysm-sidecar-config"
```

You can use both annotations together. The Secret loads first, then the ConfigMap.

The Secret or ConfigMap must exist before the RGW deployment is created or updated. Otherwise, pod startup fails.

### Step 8: PodMonitor for Prometheus

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: prysm-ops-log
  namespace: rook-ceph
  labels:
    prometheus: kube-prometheus
spec:
  selector:
    matchLabels:
      app: rook-ceph-rgw
  podMetricsEndpoints:
    - port: prysm-metrics
      interval: 60s
      path: /metrics
```

### Step 9: Verify

```bash
# Check webhook pods
kubectl -n webhook get pods

# Confirm RGW pods have the sidecar
kubectl -n rook-ceph get pods -l app=rook-ceph-rgw -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .spec.containers[*]}{.name}{" "}{end}{"\n"}{end}'

# Check sidecar logs
kubectl -n rook-ceph logs <rgw-pod-name> -c prysm-sidecar --tail=20

# Test metrics
kubectl -n rook-ceph port-forward <rgw-pod-name> 9090:9090 &
curl -s http://localhost:9090/metrics | grep radosgw_
```

## RabbitMQ audit trail (optional)

The ops-log producer can publish CADF (Cloud Auditing Data Federation) events to RabbitMQ. Downstream consumers like Hermes process these for audit and compliance.

### Step A: Deploy RabbitMQ

Skip this if you already have RabbitMQ running.

```bash
# Deploy RabbitMQ using your organization's standard method
# (operator, Helm chart, standalone -- whatever you use)
```

### Step B: Create the audit queue

Make sure the target queue exists. Default name: `keystone.notifications.info`.

### Step C: Configure the sidecar

Create or update the Secret with RabbitMQ credentials:

```bash
kubectl -n rook-ceph create secret generic prysm-sidecar-env \
  --from-literal=AUDIT_ENABLED="true" \
  --from-literal=AUDIT_RABBITMQ_URL="amqp://user:password@rabbitmq.rook-ceph.svc:5672" \
  --from-literal=AUDIT_QUEUE_NAME="keystone.notifications.info" \
  --from-literal=AUDIT_QUEUE_SIZE="20" \
  --from-literal=AUDIT_DEBUG="false" \
  --dry-run=client -o yaml | kubectl apply -f -
```

### Step D: Restart RGW pods

```bash
kubectl -n rook-ceph rollout restart deployment -l app=rook-ceph-rgw
```

### Step E: Verify audit events

```bash
# Check sidecar logs for audit activity
kubectl -n rook-ceph logs <rgw-pod-name> -c prysm-sidecar --tail=30 | grep -i audit

# Check audit metrics
curl -s http://localhost:9090/metrics | grep audittools
# audittools_successful_submissions - successful publishes
# audittools_failed_submissions - failed publishes (retried)
```

### How the audit trail works

- **Non-blocking.** Audit publishing never stalls log processing.
- **Buffered.** 20-event internal queue (configurable via `AUDIT_QUEUE_SIZE`).
- **Auto-retry.** Failed events retry every minute.
- **Graceful startup.** If RabbitMQ is unreachable at boot, the producer starts with a no-op auditor and logs a warning.
- **Connection recovery.** If RabbitMQ drops mid-operation, events buffer in memory and flush when it comes back.

**Watch out:** retries are unbounded and buffered events are not capped. During long RabbitMQ outages, memory grows with the backlog. Monitor `audittools_failed_submissions` and RabbitMQ availability.

### CADF event format

Each S3 operation produces a CADF event with:

| Field | Content |
|-------|---------|
| Initiator | Keystone user, project, domain, roles, application credentials |
| Target | Object, bucket, or account (depends on the operation) |
| Action | Mapped from the RadosGW operation (see table below) |
| Outcome | `success` or `failure` based on HTTP status |
| Observer | `prysm-ops-log` service ID |

#### RadosGW operation to CADF action

| RadosGW operation | CADF action |
|-------------------|-------------|
| `list_buckets`, `list_bucket` | `read/list` |
| `get_obj`, `get_bucket_info`, `stat_bucket`, `stat_account` | `read` |
| `put_obj`, `create_bucket`, `bulk_upload`, `restore_obj` | `create` |
| `delete_obj`, `delete_bucket`, `multi_object_delete`, `bulk_delete` | `delete` |
| `copy_obj` | `update/copy` |
| `post_obj` | `update` |

## Environment variables

### Core

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_FILE_PATH` | RGW ops-log file path | |
| `SOCKET_PATH` | Unix socket for live ops logs | |
| `MAX_LOG_FILE_SIZE` | Max log file size (MB) before rotation | |
| `LOG_RETENTION_DAYS` | Days to keep rotated logs | |
| `TRUNCATE_LOG_ON_START` | Rotate log at startup | `false` |
| `PROMETHEUS_ENABLED` | Enable metrics endpoint | `false` |
| `PROMETHEUS_PORT` | HTTP port for metrics | `8080` |
| `PROMETHEUS_INTERVAL` | Metrics update interval (seconds) | |
| `IGNORE_ANONYMOUS_REQUESTS` | Skip anonymous requests in metrics | `false` |
| `LOG_TO_STDOUT` | Print parsed entries to stdout | `false` |

### Audit trail

| Variable | Description | Default |
|----------|-------------|---------|
| `AUDIT_ENABLED` | Publish to RabbitMQ | `false` |
| `AUDIT_RABBITMQ_URL` | Connection URL (`amqp://host:port`) | |
| `AUDIT_RABBITMQ_USERNAME` | Username; overrides URL userinfo (e.g. from Vault) | |
| `AUDIT_RABBITMQ_PASSWORD` | Password; overrides URL userinfo | |
| `AUDIT_QUEUE_NAME` | Queue name (`dataplane.audit` → durable queue) | `keystone.notifications.info` |
| `AUDIT_QUEUE_SIZE` | Internal event buffer size | `20` |
| `AUDIT_DEBUG` | Log published events | `false` |
| `AUDIT_REQUIRE_TENANT` | Drop events lacking `project_id`/`domain_id` (counted in `prysm_audit_events_dropped_total`) | `true` |
| `AUDIT_OBSERVER_NAME` | CADF observer name (storage service) | `radosgw` |
| `AUDIT_REGION` | Static region stamped on events (empty = off) | |
| `AUDIT_INCLUDE_READS` | Audit reads (get_/head_/stat_/list_ prefixed) too; false = mutations-only | `true` |
| `AUDIT_SKIP_BUCKETS` | Buckets excluded from audit (comma-list, loop prevention) | `hermes` |
| `AUDIT_ALLOW_DOMAINS` | Keystone domains (ID or name, comma-list) to audit; if set, only these are published (counted as `domain_filtered`) | |
| `AUDIT_DENY_DOMAINS` | Keystone domains (ID or name, comma-list) excluded from audit; takes precedence over `AUDIT_ALLOW_DOMAINS` | |

### Metrics tracking

Set `TRACK_EVERYTHING=true` to turn on all metrics, or pick what you need:

| Variable | What it tracks |
|----------|---------------|
| `TRACK_EVERYTHING` | All of the below (shortcut) |
| `TRACK_REQUESTS_DETAILED` | Requests with full labels |
| `TRACK_REQUESTS_PER_USER` | Requests per user |
| `TRACK_REQUESTS_PER_BUCKET` | Requests per bucket |
| `TRACK_REQUESTS_PER_TENANT` | Requests per tenant |
| `TRACK_LATENCY_DETAILED` | Latency histograms with full labels |
| `TRACK_LATENCY_PER_METHOD` | Latency per HTTP method |
| `TRACK_LATENCY_PER_BUCKET` | Latency per bucket |
| `TRACK_ERRORS_PER_USER` | Errors per user |
| `TRACK_ERRORS_BY_CATEGORY` | Errors by category (timeout, connection, client, server) |
| `TRACK_TIMEOUT_ERRORS` | Timeout errors (408, 504, 598, 499) |
| `TRACK_BYTES_SENT_PER_BUCKET` | Bytes sent per bucket |
| `TRACK_BYTES_RECEIVED_PER_BUCKET` | Bytes received per bucket |

Full list of 60+ tracking variables: [environment variable reference](../pkg/producers/opslog/README.md).

### Recommended presets

**Minimal production:**
```
TRACK_LATENCY_PER_METHOD=true
TRACK_REQUESTS_PER_TENANT=true
TRACK_ERRORS_PER_USER=true
```

**Full production:**
```
TRACK_LATENCY_DETAILED=true
TRACK_LATENCY_PER_METHOD=true
TRACK_REQUESTS_PER_USER=true
TRACK_REQUESTS_PER_BUCKET=true
TRACK_ERRORS_PER_USER=true
TRACK_BYTES_SENT_PER_BUCKET=true
```

Be careful with `TRACK_EVERYTHING`. It creates many time series, which hits Prometheus storage and query performance.

## Metrics

Full table: [metrics reference](../pkg/producers/opslog/README.md#metrics-collected). Highlights:

| Metric | Type | Description |
|--------|------|-------------|
| `radosgw_total_requests` | Counter | Total requests (full labels) |
| `radosgw_total_requests_per_tenant` | Counter | Requests per tenant |
| `radosgw_bytes_sent` | Counter | Bytes sent |
| `radosgw_bytes_received` | Counter | Bytes received |
| `radosgw_errors_detailed` | Counter | Errors with full labels |
| `radosgw_timeout_errors` | Counter | Timeout errors (useful for OSD detection) |
| `radosgw_errors_by_category` | Counter | Errors by category |
| `radosgw_requests_duration` | Histogram | Request latency distribution |
| `audittools_successful_submissions` | Counter | Successful audit publishes |
| `audittools_failed_submissions` | Counter | Failed audit publishes |
