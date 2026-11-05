// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import "strings"

// SLIOperation classifies RGW operations into the ADR-defined operation categories.
type SLIOperation string

const (
	SLIOperationGet       SLIOperation = "get"
	SLIOperationPut       SLIOperation = "put"
	SLIOperationList      SLIOperation = "list"
	SLIOperationDelete    SLIOperation = "delete"
	SLIOperationHead      SLIOperation = "head"
	SLIOperationMultipart SLIOperation = "multipart"
)

// SLIProtocol represents the access protocol (S3 or Swift).
type SLIProtocol string

const (
	SLIProtocolS3    SLIProtocol = "s3"
	SLIProtocolSwift SLIProtocol = "swift"
)

// classifySLIOperation maps RGW operation names to SLI operation categories.
// Returns the operation category and whether it's a recognized SLI-relevant operation.
//
// Only data-plane operations are included in the SLI: get, put, list, delete,
// head, and multipart. Control-plane operations (create_bucket, delete_bucket,
// put_bucket_acl, put_bucket_policy, get_bucket_versioning, lifecycle, cors,
// tagging, encryption, etc.) are deliberately excluded — they return false and
// never enter the SLI numerator or denominator. This matches AWS S3 / GCP Cloud
// Storage SLA practice, where the availability metric covers object-level data
// operations, not bucket management. A 5xx on an excluded operation is still
// visible in RGW's native error counters and the audit trail, but does not
// affect the availability ratio.
//
// Operation names are the exact name() return values from RGW's C++ RGWOp
// subclasses in rgw_op.h. See docs/ceph/rgw-ops-reference.md for the
// exhaustive list.
//
// Notable RGW quirks / format differences:
//   - HEAD-on-object may be logged as "get_obj" (RGWGetObj),
//     depending on RGW version/log format.
//   - Swift account HEAD/stat may appear as "stat_account".
//   - Multipart part uploads go through RGWPutObj → "put_obj", not a
//     distinct "upload_part".
func classifySLIOperation(operation string) (SLIOperation, bool) {
	switch strings.ToLower(operation) {
	// GET — object retrieval.
	// Includes HEAD-on-object which RGW handles via RGWGetObj → "get_obj".
	case "get_obj":
		return SLIOperationGet, true

	// PUT — object writes.
	// post_obj: Swift form POST / S3 browser uploads (RGWPostObj).
	// copy_obj: server-side copy (RGWCopyObj).
	// restore_obj: S3 RestoreObject (RGWRestoreObj).
	// bulk_upload: Swift bulk upload of tar archive (RGWBulkUploadOp).
	case "put_obj", "post_obj", "copy_obj", "restore_obj", "bulk_upload":
		return SLIOperationPut, true

	// DELETE — object removal.
	// multi_object_delete: S3 multi-object delete (RGWDeleteMultiObj).
	// bulk_delete: Swift bulk delete (RGWBulkDelete).
	case "delete_obj", "multi_object_delete", "bulk_delete":
		return SLIOperationDelete, true

	// LIST — bucket and account listing.
	// list_bucket: list objects in a bucket (RGWListBucket).
	// list_buckets: list all buckets / Swift account listing (RGWListBuckets).
	case "list_bucket", "list_buckets":
		return SLIOperationList, true

	// HEAD — metadata-only requests (stat operations).
	// stat_bucket: HEAD bucket (RGWStatBucket).
	// stat_account: HEAD account, Swift only (RGWStatAccount).
	case "stat_bucket", "stat_account":
		return SLIOperationHead, true

	// MULTIPART — multipart upload lifecycle.
	// init_multipart: initiate (RGWInitMultipart).
	// complete_multipart: complete (RGWCompleteMultipart).
	// abort_multipart: abort (RGWAbortMultipart).
	// list_multipart: list parts of an in-progress upload (RGWListMultipart).
	// list_bucket_multiparts: list all in-progress multipart uploads in a bucket (RGWListBucketMultiparts).
	case "init_multipart", "complete_multipart", "abort_multipart",
		"list_multipart", "list_bucket_multiparts":
		return SLIOperationMultipart, true

	default:
		return "", false
	}
}

// detectProtocol determines the access protocol from the URI of an RGW
// ops-log entry.
//
// In the production JSON ops-log (written by rgw_ops_log_file_path) the
// operation field does NOT carry a protocol prefix — it is just
// "list_bucket", "get_obj", etc. for both S3 and Swift requests. The
// protocol is instead visible in the URI: Swift requests go through
// "/swift/v1/..." whereas S3 requests use "/bucket/key..." paths.
func detectProtocol(uri string) SLIProtocol {
	// The URI field has the form
	// "GET /swift/v1/AUTH_<tenant>/container?limit=30 HTTP/1.1".
	// Extract the request target (the path between the HTTP method and the
	// protocol version) and match the "/swift/v1" path prefix exactly, so
	// that S3 requests whose bucket or object key merely contains
	// "swift/v1" (e.g. "GET /mybucket/swift/v1/key HTTP/1.1") are not
	// misclassified as Swift. The match is case-sensitive; the Swift
	// endpoint is defined as lowercase "/swift/v1".
	target := requestTarget(uri)
	if target == "" {
		return SLIProtocolS3
	}
	// Strip the query string before matching the path prefix.
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target = target[:i]
	}
	if target == "/swift/v1" || strings.HasPrefix(target, "/swift/v1/") {
		return SLIProtocolSwift
	}

	return SLIProtocolS3
}

// requestTarget extracts the request target (path with optional query) from
// an ops-log URI line of the form "METHOD /path HTTP/x.y". It tolerates a
// missing protocol-version suffix and a bare path without a method.
//
// The ops-log "uri" field is built in rgw_log.cc from the raw CGI
// REQUEST_URI env value, which — unlike req_info::request_uri — is NOT run
// through get_abs_path(). RFC 7230 §5.3.2 permits absolute-form request
// targets ("GET http://host/swift/v1/... HTTP/1.1") and the Beast frontend
// preserves them verbatim, so an absolute-form target can reach this parser.
// We therefore mirror RGW's get_abs_path() and reduce an absolute-form target
// to its abs_path before the protocol prefix check, so a Swift request sent
// in absolute form is not misclassified as S3.
func requestTarget(uri string) string {
	// Drop the trailing protocol version (e.g. " HTTP/1.1").
	if i := strings.LastIndex(uri, " HTTP/"); i >= 0 {
		uri = uri[:i]
	}
	// Drop the leading HTTP method (up to the first space).
	if i := strings.IndexByte(uri, ' '); i >= 0 {
		uri = uri[i+1:]
	}
	return toAbsPath(strings.TrimSpace(uri))
}

// toAbsPath reduces an absolute-form request target (scheme://authority/path)
// to its abs_path, mirroring RGW's get_abs_path() in rgw_common.cc. An
// origin-form target (already starting with "/") or an unrecognized value is
// returned unchanged. Only the schemes RGW itself recognizes are stripped
// (http, https, ws, wss); anything else is left intact so a bucket/key that
// merely contains "://" is never mangled.
func toAbsPath(target string) string {
	if target == "" || target[0] == '/' {
		return target
	}
	for _, scheme := range []string{"http://", "https://", "ws://", "wss://"} {
		if !strings.HasPrefix(target, scheme) {
			continue
		}
		// Find the first '/' after the authority to get the abs_path.
		if i := strings.IndexByte(target[len(scheme):], '/'); i >= 0 {
			return target[len(scheme)+i:]
		}
		// Absolute URI with no path component (e.g. "http://host") — the
		// effective path is "/".
		return "/"
	}
	return target
}

// defaultSLIErrorCodeAllowlist is the default set of RGW error codes whose
// requests count against the availability SLO (status_class "5xx"). It mirrors
// the AWS S3 SLA error definition verbatim: only InternalError and
// ServiceUnavailable are errors for SLA purposes.
//
// Notably absent (and therefore excluded from the SLO numerator):
//   - "SlowDown"            — S3 throttling (503), routed to the "throttled" class
//   - "NotImplemented"      — 501, the operation is not supported; not a
//     backend availability failure
//   - "InsufficientCapacity" — 507 (EDQUOT/ENOSPC), tenant quota or cluster
//     capacity exhaustion; not a service-side fault in the SLA sense
//   - "UnknownError"        — RGW's catch-all for unmapped errnos (see the
//     fallback in set_req_state_err, rgw_common.cc); add it to the allowlist
//     if you want unclassifiable internal errors counted too
//
// RGW protocol tables share these code strings: Swift requests fall through
// rgw_http_swift_errors to rgw_http_s3_errors for every 5xx errno, so one
// allowlist covers both protocols (only ERR_RATE_LIMITED differs: Swift
// returns 498 "Rate Limited", handled by the throttled class via status).
var defaultSLIErrorCodeAllowlist = []string{"InternalError", "ServiceUnavailable"}

// normalizeErrorCodeAllowlist converts a configured allowlist into a lookup
// set. Entries are whitespace-trimmed and empty entries are dropped; an empty
// result falls back to the default allowlist. Matching is case-sensitive —
// RGW error codes are exact CamelCase strings ("SlowDown", not "slowdown").
func normalizeErrorCodeAllowlist(codes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		if c = strings.TrimSpace(c); c != "" {
			set[c] = struct{}{}
		}
	}
	if len(set) == 0 {
		for _, c := range defaultSLIErrorCodeAllowlist {
			set[c] = struct{}{}
		}
	}
	return set
}

// statusClass maps an ops-log entry's HTTP status and S3/Swift error code to a
// status class label, gated by the SLI error-code allowlist.
//
// Classes:
//
//   - "throttled": 429, Swift 498, and S3 503 + "SlowDown" (RGW rate_limit()
//     and the dmclock scheduler). Never counts against availability.
//   - "5xx": server errors whose error_code is in the allowlist (default:
//     InternalError, ServiceUnavailable — the AWS S3 SLA error set). Also a
//     5xx with an *empty* error_code: RGW always logs a code for failed
//     requests (rgw_log.cc), so an empty code means degraded telemetry, not a
//     success — it is counted to fail safe rather than let missing data look
//     like availability.
//   - "excluded_5xx": server errors whose error_code is NOT in the allowlist
//     (NotImplemented 501, InsufficientCapacity 507, UnknownError, future
//     codes). Visible in the metric but excluded from the SLO numerator.
//   - "1xx"…"4xx", "unknown": as before.
func statusClass(status, errorCode string, allowlist map[string]struct{}) string {
	switch {
	case status == "429",
		status == "498",
		status == "503" && errorCode == "SlowDown":
		return "throttled"
	}
	if len(status) == 3 &&
		status[0] >= '1' && status[0] <= '5' &&
		status[1] >= '0' && status[1] <= '9' &&
		status[2] >= '0' && status[2] <= '9' {
		class := string(status[0]) + "xx"
		if class != "5xx" {
			return class
		}
		// 5xx: only allowlisted error codes count against the availability
		// SLO. EBUSY and ERR_SERVICE_UNAVAILABLE both surface as
		// "ServiceUnavailable", so the default allowlist covers both.
		if errorCode == "" {
			return "5xx" // fail-safe: count unattributable server errors
		}
		if _, counted := allowlist[errorCode]; counted {
			return "5xx"
		}
		return "excluded_5xx"
	}
	return "unknown"
}

// observeSLI records per-tenant SLI request count and latency metrics.
// The SLI is keyed by tenant (not bucket), with protocol and operation labels.
// Anonymous requests (tenant="none") are excluded from the SLI.
func observeSLI(logEntry S3OperationLog, tenant string) {
	if globalSLICollector == nil {
		return
	}

	sliOperation, ok := classifySLIOperation(logEntry.Operation)
	if !ok {
		return
	}

	// On clusters without Keystone (pure S3 / Local auth, e.g. Rook), the
	// ops-log user carries no "user$tenant" suffix, so extractUserAndTenant
	// resolves every request to tenant="none" and the SLI would stay empty
	// forever. Fall back to the (non-anonymous) S3 user as the tenant: the
	// anonymous filter (IGNORE_ANONYMOUS_REQUESTS) already drops anonymous
	// traffic before metrics are updated, so the fallback cannot pollute the
	// SLI with anonymous series.
	if tenant == "none" {
		tenant = logEntry.User
	}

	protocol := detectProtocol(logEntry.URI)

	globalSLICollector.observeCounter(
		tenant,
		string(protocol),
		string(sliOperation),
		statusClass(logEntry.HTTPStatus, logEntry.ErrorCode, globalSLICollector.errorCodeAllowlist),
	)

	// Observe latency — TotalTime is in milliseconds; sub-ms requests report 0
	// which is valid and lands in the le=0.05 bucket. Only skip negative values
	// (which would indicate a corrupted log entry).
	if logEntry.TotalTime >= 0 {
		latencySec := float64(logEntry.TotalTime) / 1000.0
		globalSLICollector.observeLatency(
			tenant,
			string(protocol),
			string(sliOperation),
			latencySec,
		)
	}
}
