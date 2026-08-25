# Prysm Kubernetes Mutating Webhook for RADOSGW Sidecar Injection

This is a Kubernetes **Mutating Admission Webhook** designed to automatically
inject a **Prysm sidecar** into **RADOSGW deployments** managed by Rook-Ceph.
The sidecar container scans **RGW operation logs** and exposes **Prometheus
metrics**.

## Features

- **Automatic Sidecar Injection**: Detects `rook-ceph-rgw` deployments and
  injects a **Prysm sidecar**.
- **Prometheus Metrics**: Extracts metrics from `rgw-ops-logs` and serves them
  on port **9090**.
- **Dynamic Image Configuration**: Supports configuring the sidecar image via
  the `SIDECAR_IMAGE` environment variable.
- **Cert-Manager Integration**: Uses `cert-manager` to generate TLS
  certificates, with **automatic CA bundle injection**.
- **Secure Webhook**: Runs on port **8443** and validates incoming deployments.

---

## **Automatic Sidecar Injection**
The webhook **automatically detects** RADOSGW (`rook-ceph-rgw`) deployments and
injects a **Prysm sidecar** container. It ensures that only specific RADOSGW
instances are modified by checking **a predefined set of labels**.

### **Label Requirements**
To be **eligible for mutation**, a deployment **must have the following
labels**:

| Label | Description |
|-------|-------------|
| `app: rook-ceph-rgw` | Identifies the deployment as an RGW (RADOS Gateway) instance. |
| `app.kubernetes.io/component: cephobjectstores.ceph.rook.io` | Confirms it belongs to the Ceph Object Store component in Rook. |
| `app.kubernetes.io/created-by: rook-ceph-operator` | Ensures that the deployment was created by the Rook-Ceph Operator. |
| `app.kubernetes.io/managed-by: rook-ceph-operator` | Ensures the deployment is managed by Rook. |
| `prysm-sidecar: "yes"` | Enables Prysm sidecar injection _(must be set in `CephObjectStore.spec.gateway.labels`)_. |

>**Important:** The `prysm-sidecar: "yes"` label must be **defined in the Rook CephObjectStore configuration** under `spec.gateway.labels`. Example:

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

If this label is not set, the webhook will not modify the deployment.

#### **Sidecar Injection Process**
1.	The webhook listens for CREATE and UPDATE operations on Deployment
	resources.
2.	When a new or updated deployment matches the required labels, the
	webhook inspects its pod specification.
3.  If the **Prysm sidecar is missing**, it is **automatically injected** with
    the following configuration:
  - **Container Name**: `prysm-sidecar`
  - **Image**: Defined by `SIDECAR_IMAGE` environment variable.
  - **Args**:
    ```sh
    local-producer ops-log --log-file=/var/log/ceph/ops-log.log --max-log-file-size=10 --prometheus=true --prometheus-port=9090 -v=info
    ```
  - **Ports**:
    - `9090/TCP` (Prometheus metrics endpoint)
  - **Volume Mounts**:
    - `/etc/ceph` (Rook configuration)
    - `/run/ceph` (Ceph daemon sockets)
    - `/var/log/ceph` (RGW operation logs)
    - `/var/lib/ceph/crash` (Crash logs)
  - **Environment Variables**:
    - `POD_NAME`: Auto-populated with the pod’s name.
4.  If a **Prysm sidecar already exists**, the webhook **updates it** to ensure
    consistency with the latest configuration.
5.	The modified deployment is then approved and applied to the cluster.

This ensures consistent, automated sidecar injection into selected
rook-ceph-rgw instances, allowing **real-time monitoring of RGW operations**.

---

## Configure Sidecar via Secret or ConfigMap

The webhook supports injecting **environment variables** into the Prysm sidecar
using either a **Secret** or a **ConfigMap**.  This allows each RADOSGW
deployment to customize the sidecar's behavior independently.

### Option 1: Use a Secret

Add this annotation to your CephObjectStore or RADOSGW deployment:

```yaml
   annotations:
     prysm-sidecar/sidecar-env-secret: "prysm-sidecar-env"
```
The specified secret must exist in the same namespace and look like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prysm-sidecar-env
  namespace: rook-ceph
type: Opaque
stringData:
  LOG_FILE_PATH: "/var/log/ceph/ops-log.log"
  MAX_LOG_FILE_SIZE: "10"
  PROMETHEUS_PORT: "9090"
  IGNORE_ANONYMOUS_REQUESTS: "true"
  TRACK_REQUESTS_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_METHOD_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_OPERATION_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_STATUS_PER_BUCKET: "true"
  TRACK_BYTES_SENT_PER_BUCKET: "true"
  TRACK_BYTES_RECEIVED_PER_BUCKET: "true"
  TRACK_LATENCY_PER_BUCKET: "true"
  TRACK_LATENCY_PER_BUCKET_AND_METHOD: "true"
  TRACK_ERRORS_PER_BUCKET: "true"
  TRACK_ERRORS_BY_CATEGORY: "true"
  TRACK_TIMEOUT_ERRORS: "true"
  TRACK_BUCKET_SLO: "true"
```

### Option 2: Use a ConfigMap

Alternatively, you can use a ConfigMap by setting this annotation:

```yaml
   annotations:
     prysm-sidecar/sidecar-env-configmap: "prysm-sidecar-config"
```
Example ConfigMap:

```yaml
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
  TRACK_REQUESTS_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_METHOD_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_OPERATION_PER_BUCKET: "true"
  TRACK_REQUESTS_BY_STATUS_PER_BUCKET: "true"
  TRACK_BYTES_SENT_PER_BUCKET: "true"
  TRACK_BYTES_RECEIVED_PER_BUCKET: "true"
  TRACK_LATENCY_PER_BUCKET: "true"
  TRACK_LATENCY_PER_BUCKET_AND_METHOD: "true"
  TRACK_ERRORS_PER_BUCKET: "true"
  TRACK_ERRORS_BY_CATEGORY: "true"
  TRACK_TIMEOUT_ERRORS: "true"
  TRACK_BUCKET_SLO: "true"
```
### You Can Use Both

If both annotations are set, the sidecar will receive **both** sources via
envFrom, in the order:
1.	Secret (if specified)
2.	ConfigMap (if specified)

This allows sensitive data to be stored in Secrets, while general config can go
in a ConfigMap.

### Benefits

- Each RADOSGW instance can have its own metrics configuration
- Keeps configuration clean and modular
- Avoids hardcoding environment variables into the webhook

### Audit Trail (RabbitMQ)

The sidecar can publish CADF audit events to RabbitMQ. All audit settings are
configurable via environment variables, so they can be supplied through the
Secret above without changing the webhook or the sidecar command line. Because
the connection URL contains credentials, store these in a **Secret** (not a
ConfigMap).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prysm-sidecar-env
  namespace: rook-ceph
type: Opaque
stringData:
  AUDIT_ENABLED: "true"
  AUDIT_RABBITMQ_URL: "amqp://user:password@rabbitmq.example:5672/"
  AUDIT_QUEUE_NAME: "keystone.notifications.info"   # optional, this is the default
  AUDIT_QUEUE_SIZE: "20"                            # optional, internal buffer size
  AUDIT_DEBUG: "false"                              # optional, log published events
```

| Variable                  | Description                                          | Default                          |
|---------------------------|------------------------------------------------------|----------------------------------|
| `AUDIT_ENABLED`           | Publish CADF audit events to RabbitMQ               | `false`                          |
| `AUDIT_RABBITMQ_URL`      | AMQP connection URL (`amqp://host:port/`)           | _empty_                          |
| `AUDIT_RABBITMQ_USERNAME` | Username; overrides any userinfo in the URL          | _empty_                          |
| `AUDIT_RABBITMQ_PASSWORD` | Password; overrides any userinfo in the URL          | _empty_                          |
| `AUDIT_QUEUE_NAME`        | Target queue (see durability note below)             | `keystone.notifications.info`    |
| `AUDIT_QUEUE_SIZE`        | Internal event buffer size                           | `20`                             |
| `AUDIT_DEBUG`             | Log every published event (verbose)                  | `false`                          |
| `AUDIT_REQUIRE_TENANT`    | Drop events lacking a project_id/domain_id (counted) | `true`                           |
| `AUDIT_OBSERVER_NAME`     | CADF observer name (storage service)                 | `radosgw`                        |
| `AUDIT_REGION`            | Static region stamped on events (empty = off)        | _empty_                          |
| `AUDIT_INCLUDE_READS`     | Audit reads (get/head/list) too; false = mutations-only | `true`                        |
| `AUDIT_SKIP_BUCKETS`      | Buckets excluded from audit (comma-list, loop prevention) | `hermes`                    |
| `AUDIT_ALLOW_DOMAINS`     | Keystone domains (ID or name, comma-list) to audit; only these published when set | _empty_ |
| `AUDIT_DENY_DOMAINS`      | Keystone domains (ID or name, comma-list) excluded; precedes allow-list | _empty_          |

> These are non-sensitive — put them in the ConfigMap (`sidecarEnvConfig.config`),
> not the Secret. Only the RabbitMQ credentials belong in the Secret.

> If `AUDIT_ENABLED=true` but `AUDIT_RABBITMQ_URL` is empty, the sidecar logs a
> warning and falls back to a no-op auditor — log processing is never blocked.

> **Durable queue / log-router:** the underlying `go-bits/audittools` library
> declares the queue **durable** only when `AUDIT_QUEUE_NAME` is exactly
> **`dataplane.audit`**; any other name is a transient queue. The dataplane
> audit log-router consumes `dataplane.audit` and requires a durable queue, so
> set `AUDIT_QUEUE_NAME: "dataplane.audit"` for it to connect. Note: a durable
> queue survives a broker restart, but the messages themselves are still
> published transient (not persisted). If the queue already exists with a
> different durability flag, delete it first — RabbitMQ rejects a redeclare with
> `406 PRECONDITION_FAILED`.

#### Separate username / password (e.g. from Vault)

The username and password can be supplied independently of the URL via
`AUDIT_RABBITMQ_USERNAME` / `AUDIT_RABBITMQ_PASSWORD`. When set, they are
composed into the URL's userinfo at runtime and **override** any credentials
embedded in `AUDIT_RABBITMQ_URL`. This lets the two values come from two
separate sources (such as two Vault entries) without string-building a
connection URL:

```yaml
stringData:
  AUDIT_ENABLED: "true"
  AUDIT_RABBITMQ_URL: "amqp://rabbitmq.example:5672/"   # no credentials in the URL
  AUDIT_RABBITMQ_USERNAME: "audit"
  AUDIT_RABBITMQ_PASSWORD: "s3cr3t"
```

##### Sourcing the credentials from HashiCorp Vault

The sidecar itself does **not** talk to Vault — keep it Vault-unaware. Instead,
use an operator such as the
[External Secrets Operator](https://external-secrets.io/) (or the HashiCorp
Vault Secrets Operator) to project your two Vault entries into the Secret that
the webhook injects. Example `ExternalSecret`:

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: prysm-sidecar-env
  namespace: rook-ceph
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: prysm-sidecar-env   # the Secret referenced by the sidecar-env-secret annotation
    template:
      type: Opaque
      data:
        AUDIT_ENABLED: "true"
        AUDIT_RABBITMQ_URL: "amqp://rabbitmq.example:5672/"
        AUDIT_RABBITMQ_USERNAME: "{{ .username }}"
        AUDIT_RABBITMQ_PASSWORD: "{{ .password }}"
  data:
    - secretKey: username
      remoteRef:
        key: secret/data/rabbitmq/audit
        property: username
    - secretKey: password
      remoteRef:
        key: secret/data/rabbitmq/audit
        property: password
```

The operator renders a native `Secret`; the webhook injects it via `envFrom`
exactly as above. Credentials never land in the Deployment spec or the webhook.

---
### Important Notes
> The referenced Secret or ConfigMap must exist before the deployment is
> created, or pod startup may fail.

---

## **Environment Variables**

| Variable         | Description                                      | Default |
|-----------------|--------------------------------------------------|---------|
| `WEBHOOK_PORT`  | Port for the webhook server                      | `8443`  |
| `SIDECAR_IMAGE` | The Prysm sidecar image (use a specific version tag) | _None_  |

### **Best Practice: Use Explicit Version Tags**
It is **strongly recommended** to use a **specific version tag** instead of
`latest`. This ensures:
- **Predictability**: Prevents unexpected changes due to automatic image
  updates.
- **Security**: Avoids potential vulnerabilities in newly pushed images.
- **Stability**: Ensures compatibility with the webhook’s configuration.

#### **Example: Setting a Fixed Version**
```yaml
env:
  - name: SIDECAR_IMAGE
    value: "ghcr.io/cobaltcore-dev/prysm:v1.2.3"
```

This ensures that **every deployment uses the same tested and verified
version** of the Prysm sidecar.

⸻

## **Deployment**

#### **Deploy cert-manager Resources**

The webhook **uses cert-manager** to **generate TLS certificates** and
**automatically inject the CA bundle** into the MutatingWebhookConfiguration.
```yaml
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
```

---

#### **Deploy the Webhook Server**
```yaml
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
        image: "ghcr.io/cobaltcore-dev/prysm-wh:v1.2.3"
        ports:
        - containerPort: 8443
        volumeMounts:
        - name: certs
          mountPath: "/certs"
          readOnly: true
        env:
        - name: SIDECAR_IMAGE
          value: "ghcr.io/cobaltcore-dev/prysm:v1.2.3"
        imagePullPolicy: Always
      volumes:
      - name: certs
        secret:
          secretName: prysm-webhook-cert
```

⸻

## **Deploy the Mutating Webhook Configuration**

```yaml
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
      - operations: ["CREATE","UPDATE"]
        apiGroups: ["apps"]
        apiVersions: ["v1"]
        resources: ["deployments"]
```
For more information, visit the [Prysm ops-log local-producer](https://github.com/cobaltcore-dev/prysm/blob/main/pkg/producers/opslog/README.md) documentation.
