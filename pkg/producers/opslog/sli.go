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
//   - HEAD-on-object may be logged as "get_obj" (RGWGetObj) or "head_obj",
//     depending on RGW version/log format.
//   - HEAD on a bucket may be logged as "head_bucket" or "stat_bucket".
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
	// We look for "/swift/v1/" anywhere in the URI to keep it simple
	// and resilient to variations in HTTP method or query parameters.
	lowerURI := strings.ToLower(uri)
	if strings.Contains(lowerURI, "/swift/v1/") || strings.Contains(lowerURI, "/swift/v1 ") {
		return SLIProtocolSwift
	}

	return SLIProtocolS3
}

func statusClass(status string) string {
	if len(status) == 3 &&
		status[0] >= '1' && status[0] <= '5' &&
		status[1] >= '0' && status[1] <= '9' &&
		status[2] >= '0' && status[2] <= '9' {
		return string(status[0]) + "xx"
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
	if tenant == "none" || !ok {
		return
	}

	protocol := detectProtocol(logEntry.URI)

	globalSLICollector.observeCounter(
		tenant,
		string(protocol),
		string(sliOperation),
		statusClass(logEntry.HTTPStatus),
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
