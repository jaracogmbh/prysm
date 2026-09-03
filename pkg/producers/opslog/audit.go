// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/sapcc/go-api-declarations/cadf"
	"github.com/sapcc/go-bits/audittools"
)

// buildObserver constructs the CADF observer that identifies the storage
// service reporting the event (the service, not the resource or the tool).
// The name is configurable and defaults to "radosgw".
func buildObserver(cfg AuditSinkConfig) audittools.Observer {
	name := cfg.ObserverName
	if name == "" {
		name = "radosgw"
	}
	return audittools.Observer{
		TypeURI: "service/storage",
		Name:    name,
		ID:      audittools.GenerateUUID(),
	}
}

// InitAuditor initializes the audit trail based on configuration.
// Returns a NullAuditor if audit sink is not configured or disabled.
func InitAuditor(ctx context.Context, cfg AuditSinkConfig, registry prometheus.Registerer) audittools.Auditor {
	if !cfg.Enabled {
		log.Info().Msg("Audit sink disabled, using NullAuditor")
		return audittools.NewNullAuditor()
	}

	if cfg.RabbitMQURL == "" || cfg.QueueName == "" {
		log.Warn().Msg("Audit sink enabled but RabbitMQ not configured, using NullAuditor")
		return audittools.NewNullAuditor()
	}

	connectionURL, err := buildRabbitMQConnectionURL(cfg.RabbitMQURL, cfg.RabbitMQUsername, cfg.RabbitMQPassword)
	if err != nil {
		log.Error().Err(err).Msg("Invalid RabbitMQ connection URL, falling back to NullAuditor")
		return audittools.NewNullAuditor()
	}

	queueSize := cfg.InternalQueueSize
	if queueSize == 0 {
		queueSize = 20 // Default from audittools
	}

	auditor, err := audittools.NewAuditor(ctx, audittools.AuditorOpts{
		ConnectionURL: connectionURL,
		QueueName:     cfg.QueueName,
		Observer:      buildObserver(cfg),
		Registry:      registry,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize auditor, falling back to NullAuditor")
		return audittools.NewNullAuditor()
	}

	log.Info().
		Str("queue", cfg.QueueName).
		Int("buffer_size", queueSize).
		Bool("debug", cfg.Debug).
		Msg("Audit trail initialized successfully")

	return auditor
}

// buildRabbitMQConnectionURL injects an explicit username/password into the
// userinfo of an AMQP connection URL. Explicit credentials override any
// already present in the URL. When both username and password are empty, the
// URL is returned unchanged. This allows the credentials to be supplied as two
// independent values (e.g. two Vault entries synced into a Secret) rather than
// embedded in a single connection string.
func buildRabbitMQConnectionURL(rawURL, username, password string) (string, error) {
	if username == "" && password == "" {
		return rawURL, nil
	}

	if username == "" && password != "" {
		return "", fmt.Errorf("invalid RabbitMQ credentials: password provided without username")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid RabbitMQ URL: %w", err)
	}

	if password != "" {
		u.User = url.UserPassword(username, password)
	} else {
		u.User = url.User(username)
	}

	return u.String(), nil
}

// regionTarget decorates a Target with a static region attachment. Region is
// a per-cluster deployment fact (the ops log has none), so it is supplied via
// config rather than derived per request. Implemented as a decorator so the
// placement can be changed easily if the audit consumer expects it elsewhere.
type regionTarget struct {
	inner  audittools.Target
	region string
}

func (t regionTarget) Render() cadf.Resource {
	resource := t.inner.Render()
	resource.Attachments = append(resource.Attachments, cadf.Attachment{
		Name:    "region",
		TypeURI: "xs:string",
		Content: t.region,
	})
	return resource
}

// withRegion wraps a Target so its rendered resource carries the region. An
// empty region returns the target unchanged (no attachment added).
func withRegion(target audittools.Target, region string) audittools.Target {
	if region == "" {
		return target
	}
	return regionTarget{inner: target, region: region}
}

// ToAuditEvent converts an S3OperationLog to an audittools.Event. The region is
// a static per-cluster value stamped onto the target (empty = not stamped).
func (opLog *S3OperationLog) ToAuditEvent(region string) (audittools.Event, error) {
	// Parse timestamp
	eventTime, err := time.Parse("2006-01-02T15:04:05.999999Z", opLog.Time)
	if err != nil {
		// Fallback to current time if parsing fails
		eventTime = time.Now()
		log.Warn().Err(err).Str("time", opLog.Time).Msg("Failed to parse ops log timestamp, using current time")
	}

	// Build HTTP request for audit context
	req, err := buildHTTPRequest(opLog)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to build HTTP request for audit event")
	}

	// Parse HTTP status code
	reasonCode, err := strconv.Atoi(opLog.HTTPStatus)
	if err != nil {
		reasonCode = 500 // Default to internal server error if parsing fails
		log.Warn().Err(err).Str("http_status", opLog.HTTPStatus).Msg("Failed to parse HTTP status")
	}

	return audittools.Event{
		Time:       eventTime,
		Request:    req,
		User:       buildUserInfo(opLog),
		ReasonCode: reasonCode,
		Action:     mapOperationToAction(opLog.Operation),
		Target:     withRegion(buildTarget(opLog), region),
	}, nil
}

// buildHTTPRequest constructs an http.Request from the ops log entry.
func buildHTTPRequest(opLog *S3OperationLog) (*http.Request, error) {
	// Parse the URI
	uri := opLog.URI
	// Extract path from "GET /path HTTP/1.1" format
	parts := strings.Fields(uri)
	path := "/"
	if len(parts) >= 2 {
		path = parts[1]
	}

	// Build URL
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	// Create request
	method := "GET" // Default
	if len(parts) >= 1 {
		method = parts[0]
	}

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("User-Agent", opLog.UserAgent)
	if opLog.Referrer != "" {
		req.Header.Set("Referer", opLog.Referrer)
	}

	// Set remote address
	req.RemoteAddr = opLog.RemoteAddr

	return req, nil
}

// isSkippedBucket reports whether operations on the given bucket must be
// excluded from audit. This breaks the Hermes loop: Hermes writes audit events
// into a per-customer WORM bucket, and those writes would otherwise generate
// new audit events. Matching is case-insensitive over a comma-separated list;
// an empty list disables the filter.
func isSkippedBucket(bucket, skipBuckets string) bool {
	if bucket == "" || skipBuckets == "" {
		return false
	}
	b := strings.ToLower(strings.TrimSpace(bucket))
	for _, name := range strings.Split(skipBuckets, ",") {
		if b == strings.ToLower(strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

// matchesAny reports whether any candidate is present in the comma-separated,
// case-insensitive token list. Empty/blank candidates never match; an empty
// list never matches. Used by the domain filter to test a domain's ID and name
// against a single AllowDomains/DenyDomains list.
func matchesAny(candidates []string, list string) bool {
	if list == "" {
		return false
	}
	for _, name := range strings.Split(list, ",") {
		token := strings.ToLower(strings.TrimSpace(name))
		if token == "" {
			continue
		}
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if strings.ToLower(strings.TrimSpace(c)) == token {
				return true
			}
		}
	}
	return false
}

// isDomainAudited reports whether an entry passes the domain filter and should
// be audited. The domain is taken from KeystoneScope.Project.Domain and matched
// (by ID or name) against the allow/deny lists. Precedence: deny wins; then, if
// the allow list is non-empty, the domain must be in it. When both lists are
// empty the filter is disabled and every entry passes. An entry without a
// Keystone scope has no domain: it fails a non-empty allow list, but passes when
// only a deny list (or neither) is configured.
func isDomainAudited(opLog *S3OperationLog, cfg AuditSinkConfig) bool {
	if cfg.AllowDomains == "" && cfg.DenyDomains == "" {
		return true
	}

	var domainID, domainName string
	if opLog.KeystoneScope != nil {
		domainID = opLog.KeystoneScope.Project.Domain.ID
		domainName = opLog.KeystoneScope.Project.Domain.Name
	}
	candidates := []string{domainID, domainName}

	if matchesAny(candidates, cfg.DenyDomains) {
		return false
	}
	if cfg.AllowDomains != "" {
		return matchesAny(candidates, cfg.AllowDomains)
	}
	return true
}

// isReadOperation reports whether an RGW operation is a read
// (get_/head_/stat_/list_ prefixed operations).
//
// Read classification is by operation name prefix and is independent of the
// CADF action mapping, so it is robust regardless of how actions are finalized.
//
// By default reads ARE included in the audit trail (--audit-include-reads
// defaults to true). Set AUDIT_INCLUDE_READS=false for mutations-only auditing.
//
// Note: RGW logs HEAD-on-bucket as "stat_bucket" and HEAD-on-account as
// "stat_account" (not "head_bucket"/"head_account"). The stat_ prefix is
// included alongside head_ for correctness.
func isReadOperation(operation string) bool {
	return strings.HasPrefix(operation, "get_") ||
		strings.HasPrefix(operation, "head_") ||
		strings.HasPrefix(operation, "stat_") ||
		strings.HasPrefix(operation, "list_")
}

// mapOperationToAction converts RadosGW operation names to CADF actions.
//
// Operation names are the exact name() return values from RGW's C++ RGWOp
// subclasses in rgw_op.h. Notable quirks:
//   - HEAD on an object is handled by RGWGetObj → logs as "get_obj", not "head_obj".
//   - HEAD on a bucket is RGWStatBucket → "stat_bucket".
//   - HEAD on an account (Swift) is RGWStatAccount → "stat_account".
func mapOperationToAction(operation string) cadf.Action {
	switch operation {
	case "list_buckets", "list_bucket":
		return "read/list"
	case "get_obj", "get_bucket_info", "stat_bucket", "stat_account":
		return "read"
	case "put_obj", "create_bucket", "bulk_upload", "restore_obj":
		return "create"
	case "delete_obj", "delete_bucket", "multi_object_delete", "bulk_delete":
		return "delete"
	case "copy_obj":
		return "update/copy"
	case "post_obj":
		return "update"
	default:
		log.Debug().Str("operation", operation).Msg("Unknown operation, using 'unknown' action")
		return cadf.UnknownAction
	}
}

// buildTarget creates a CADF Target resource from the ops log entry.
func buildTarget(opLog *S3OperationLog) audittools.Target {
	// Determine target type and ID
	if opLog.Object != "" {
		// Object-level operation
		return &ObjectTarget{
			Bucket: opLog.Bucket,
			Object: opLog.Object,
		}
	} else if opLog.Bucket != "" {
		// Bucket-level operation
		return &BucketTarget{
			Bucket: opLog.Bucket,
		}
	} else {
		// Account-level operation (e.g., list_buckets). Prefer the Keystone
		// project ID (same source as the initiator) so target and initiator
		// agree; fall back to the parsed user when no Keystone scope exists.
		projectID, _ := resolveTenant(opLog)
		return &AccountTarget{
			ProjectID: projectID,
		}
	}
}

// resolveTenant returns the project ID and domain ID that would populate the
// CADF initiator for this entry, using the same precedence as buildUserInfo:
// the Keystone scope when present, otherwise the project parsed from the user.
func resolveTenant(opLog *S3OperationLog) (projectID, domainID string) {
	if opLog.KeystoneScope != nil {
		return opLog.KeystoneScope.Project.ID, opLog.KeystoneScope.Project.Domain.ID
	}
	return extractProjectID(opLog.User), ""
}

// hasUsableTenant reports whether the entry yields a project_id or domain_id
// for the CADF initiator. The audit consumer rejects events that carry
// neither, so AUDIT_REQUIRE_TENANT uses this to drop them before publishing.
// The anonymous sentinel is treated as no tenant.
func hasUsableTenant(opLog *S3OperationLog) bool {
	projectID, domainID := resolveTenant(opLog)
	if domainID != "" {
		return true
	}
	return projectID != "" && projectID != "anonymous"
}

// extractProjectID extracts the project ID from the user field.
// User format is typically: "projectID$projectID" or just "projectID"
func extractProjectID(user string) string {
	parts := strings.Split(user, "$")
	if len(parts) > 0 {
		return parts[0]
	}
	return user
}

// buildUserInfo creates a UserInfo from the Keystone scope.
func buildUserInfo(opLog *S3OperationLog) audittools.UserInfo {
	if opLog.KeystoneScope == nil {
		// Fallback if no Keystone scope available
		return &SimpleUserInfo{
			UserID:    opLog.User,
			ProjectID: extractProjectID(opLog.User),
		}
	}

	return &KeystoneUserInfo{
		ProjectID:   opLog.KeystoneScope.Project.ID,
		ProjectName: opLog.KeystoneScope.Project.Name,
		DomainID:    opLog.KeystoneScope.Project.Domain.ID,
		DomainName:  opLog.KeystoneScope.Project.Domain.Name,
		UserID:      opLog.KeystoneScope.User.ID,
		UserName:    opLog.KeystoneScope.User.Name,
		UserDomain:  opLog.KeystoneScope.User.Domain.Name,
		Roles:       opLog.KeystoneScope.Roles,
		AppCredID:   getAppCredID(opLog.KeystoneScope.ApplicationCredential),
		AppCredName: getAppCredName(opLog.KeystoneScope.ApplicationCredential),
	}
}

func getAppCredID(appCred *KeystoneApplicationCredential) string {
	if appCred != nil {
		return appCred.ID
	}
	return ""
}

func getAppCredName(appCred *KeystoneApplicationCredential) string {
	if appCred != nil {
		return appCred.Name
	}
	return ""
}

// === Target Implementations ===

// ObjectTarget represents an object-level operation target.
type ObjectTarget struct {
	Bucket string
	Object string
}

func (t *ObjectTarget) Render() cadf.Resource {
	return cadf.Resource{
		TypeURI: "service/storage/object",
		ID:      t.Bucket + "/" + t.Object,
		Name:    t.Object,
		Attachments: []cadf.Attachment{
			{
				Name:    "bucket",
				TypeURI: "service/storage/bucket",
				Content: t.Bucket,
			},
		},
	}
}

// BucketTarget represents a bucket-level operation target.
type BucketTarget struct {
	Bucket string
}

func (t *BucketTarget) Render() cadf.Resource {
	return cadf.Resource{
		TypeURI: "service/storage/bucket",
		ID:      t.Bucket,
		Name:    t.Bucket,
	}
}

// AccountTarget represents an account-level operation target.
type AccountTarget struct {
	ProjectID string
}

func (t *AccountTarget) Render() cadf.Resource {
	return cadf.Resource{
		TypeURI: "service/storage/account",
		ID:      t.ProjectID,
		Name:    t.ProjectID,
	}
}

// === UserInfo Implementations ===

// KeystoneUserInfo represents a Keystone-authenticated user.
type KeystoneUserInfo struct {
	ProjectID   string
	ProjectName string
	DomainID    string
	DomainName  string
	UserID      string
	UserName    string
	UserDomain  string
	Roles       []string
	AppCredID   string
	AppCredName string
}

func (u *KeystoneUserInfo) AsInitiator(host cadf.Host) cadf.Resource {
	initiator := cadf.Resource{
		TypeURI:           "service/security/account/user",
		ID:                u.UserID,
		Name:              u.UserName,
		Host:              &host,
		ProjectID:         u.ProjectID,
		ProjectName:       u.ProjectName,
		DomainID:          u.DomainID,
		DomainName:        u.DomainName,
		ProjectDomainName: u.DomainName,
		Domain:            u.UserDomain,
	}

	// Add application credential if present
	if u.AppCredID != "" {
		initiator.AppCredentialID = u.AppCredID
		initiator.Attachments = []cadf.Attachment{
			{
				Name:    "application_credential_name",
				TypeURI: "xs:string",
				Content: u.AppCredName,
			},
		}
	}

	return initiator
}

// SimpleUserInfo represents a basic user without Keystone scope.
type SimpleUserInfo struct {
	UserID    string
	ProjectID string
}

func (u *SimpleUserInfo) AsInitiator(host cadf.Host) cadf.Resource {
	return cadf.Resource{
		TypeURI:   "service/security/account/user",
		ID:        u.UserID,
		Name:      u.UserID,
		Host:      &host,
		ProjectID: u.ProjectID,
	}
}
