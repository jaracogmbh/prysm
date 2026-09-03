// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"testing"

	"github.com/sapcc/go-api-declarations/cadf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRabbitMQConnectionURL verifies that explicitly supplied username and
// password (e.g. sourced from two separate Vault entries synced into a Secret)
// are composed into the AMQP connection URL's userinfo.
func TestBuildRabbitMQConnectionURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		username string
		password string
		want     string
	}{
		{
			name:   "no credentials returns url unchanged",
			rawURL: "amqp://rabbitmq.example:5672/",
			want:   "amqp://rabbitmq.example:5672/",
		},
		{
			name:     "username and password injected into bare host url",
			rawURL:   "amqp://rabbitmq.example:5672/",
			username: "audit",
			password: "s3cr3t",
			want:     "amqp://audit:s3cr3t@rabbitmq.example:5672/",
		},
		{
			name:     "username only",
			rawURL:   "amqp://rabbitmq.example:5672/",
			username: "audit",
			want:     "amqp://audit@rabbitmq.example:5672/",
		},
		{
			name:     "explicit credentials override userinfo already in url",
			rawURL:   "amqp://old:oldpass@rabbitmq.example:5672/",
			username: "audit",
			password: "s3cr3t",
			want:     "amqp://audit:s3cr3t@rabbitmq.example:5672/",
		},
		{
			name:     "special characters in password are percent-encoded",
			rawURL:   "amqp://rabbitmq.example:5672/vhost",
			username: "audit",
			password: "p@ss/w:rd",
			want:     "amqp://audit:p%40ss%2Fw%3Ard@rabbitmq.example:5672/vhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildRabbitMQConnectionURL(tt.rawURL, tt.username, tt.password)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("invalid url with credentials returns error", func(t *testing.T) {
		_, err := buildRabbitMQConnectionURL("://not a url", "u", "p")
		require.Error(t, err)
	})

	t.Run("password without username is rejected", func(t *testing.T) {
		_, err := buildRabbitMQConnectionURL("amqp://rabbitmq.example:5672/", "", "s3cr3t")
		require.Error(t, err)
	})
}

// TestHasUsableTenant verifies the tenant gate used by AUDIT_REQUIRE_TENANT:
// an event is publishable only if it yields a project_id or domain_id.
func TestHasUsableTenant(t *testing.T) {
	scope := func(projectID, domainID string) *KeystoneScope {
		return &KeystoneScope{
			Project: KeystoneProject{
				ID:     projectID,
				Domain: KeystoneDomain{ID: domainID},
			},
		}
	}

	tests := []struct {
		name  string
		opLog *S3OperationLog
		want  bool
	}{
		{"keystone scope with project id", &S3OperationLog{KeystoneScope: scope("proj-1", "dom-1")}, true},
		{"keystone scope with only domain id", &S3OperationLog{KeystoneScope: scope("", "dom-1")}, true},
		{"keystone scope with neither", &S3OperationLog{KeystoneScope: scope("", "")}, false},
		{"no scope, tenant-encoded user", &S3OperationLog{User: "tenant1$user1"}, true},
		{"no scope, bare user", &S3OperationLog{User: "user1"}, true},
		{"no scope, anonymous user", &S3OperationLog{User: "anonymous"}, false},
		{"no scope, empty user", &S3OperationLog{User: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasUsableTenant(tt.opLog))
		})
	}
}

// TestBuildTargetAccountProjectID verifies that the account-level target uses
// the Keystone project ID when present (consistent with the initiator),
// falling back to the parsed user only when no Keystone scope is available.
func TestBuildTargetAccountProjectID(t *testing.T) {
	t.Run("prefers keystone project id over parsed user", func(t *testing.T) {
		opLog := &S3OperationLog{
			User: "rgwuser", // would parse to "rgwuser"
			KeystoneScope: &KeystoneScope{
				Project: KeystoneProject{ID: "proj-hash-123"},
			},
		}
		acct, ok := buildTarget(opLog).(*AccountTarget)
		require.True(t, ok)
		assert.Equal(t, "proj-hash-123", acct.ProjectID)
	})

	t.Run("falls back to parsed user without keystone scope", func(t *testing.T) {
		opLog := &S3OperationLog{User: "tenant1$user1"}
		acct, ok := buildTarget(opLog).(*AccountTarget)
		require.True(t, ok)
		assert.Equal(t, "tenant1", acct.ProjectID)
	})
}

// TestWithRegion verifies that a configured region is stamped onto the target
// as an attachment, and that an empty region leaves the target untouched.
func TestWithRegion(t *testing.T) {
	regionOf := func(r cadf.Resource) string {
		for _, a := range r.Attachments {
			if a.Name == "region" {
				if s, ok := a.Content.(string); ok {
					return s
				}
			}
		}
		return ""
	}

	t.Run("adds region attachment", func(t *testing.T) {
		target := withRegion(&BucketTarget{Bucket: "b1"}, "qa-de-1")
		assert.Equal(t, "qa-de-1", regionOf(target.Render()))
	})

	t.Run("empty region leaves target unchanged", func(t *testing.T) {
		base := &BucketTarget{Bucket: "b1"}
		target := withRegion(base, "")
		assert.Same(t, base, target)
		assert.Equal(t, "", regionOf(target.Render()))
	})
}

// TestBuildObserver verifies the audit observer identifies the storage service
// (not the resource/tool), with a configurable name defaulting to radosgw.
func TestBuildObserver(t *testing.T) {
	t.Run("defaults to radosgw service when name unset", func(t *testing.T) {
		obs := buildObserver(AuditSinkConfig{})
		assert.Equal(t, "service/storage", obs.TypeURI)
		assert.Equal(t, "radosgw", obs.Name)
		assert.NotEmpty(t, obs.ID)
	})

	t.Run("uses configured observer name", func(t *testing.T) {
		obs := buildObserver(AuditSinkConfig{ObserverName: "ceph"})
		assert.Equal(t, "service/storage", obs.TypeURI)
		assert.Equal(t, "ceph", obs.Name)
	})
}

// TestIsReadOperation verifies read classification (get_/head_/stat_/list_
// prefixed operations) used by the optional read filter. Reads are audited by
// default for object storage, but can be excluded (mutations-only) via
// AUDIT_INCLUDE_READS=false.
func TestIsReadOperation(t *testing.T) {
	reads := []string{
		"get_obj", "stat_bucket", "stat_account", "get_bucket_info",
		"head_synthetic", // exercises the head_ prefix branch (no real RGW op uses this prefix)
		"list_buckets", "list_bucket", "get_acls", "get_bucket_policy",
		"get_lifecycle", "get_obj_tags",
	}
	mutations := []string{
		"put_obj", "create_bucket", "delete_obj", "delete_bucket", "copy_obj",
		"post_obj", "put_acls", "init_multipart", "complete_multipart",
		"abort_multipart", "multi_object_delete", "bulk_delete",
		"bulk_upload", "restore_obj",
	}

	for _, op := range reads {
		assert.True(t, isReadOperation(op), "expected %q to be a read", op)
	}
	for _, op := range mutations {
		assert.False(t, isReadOperation(op), "expected %q to be a mutation", op)
	}
}

// TestMapOperationToAction verifies the RGW operation → CADF action mapping.
// Every case in mapOperationToAction is exercised, plus the unknown fallback.
func TestMapOperationToAction(t *testing.T) {
	testCases := []struct {
		operation string
		expected  cadf.Action
	}{
		// read/list
		{"list_buckets", "read/list"},
		{"list_bucket", "read/list"},

		// read
		{"get_obj", "read"},
		{"get_bucket_info", "read"},
		{"stat_bucket", "read"},
		{"stat_account", "read"},

		// create
		{"put_obj", "create"},
		{"create_bucket", "create"},
		{"bulk_upload", "create"},
		{"restore_obj", "create"},

		// delete
		{"delete_obj", "delete"},
		{"delete_bucket", "delete"},
		{"multi_object_delete", "delete"},
		{"bulk_delete", "delete"},

		// update/copy
		{"copy_obj", "update/copy"},

		// update
		{"post_obj", "update"},

		// unknown fallback
		{"some_unknown_op", cadf.UnknownAction},
		{"init_multipart", cadf.UnknownAction},
	}

	for _, tc := range testCases {
		t.Run(tc.operation, func(t *testing.T) {
			assert.Equal(t, tc.expected, mapOperationToAction(tc.operation),
				"operation %q should map to %q", tc.operation, tc.expected)
		})
	}
}

// TestIsSkippedBucket verifies the loop-prevention filter: operations on the
// Hermes audit bucket are skipped so Hermes' own writes don't re-trigger audit.
// Matching is case-insensitive and supports a comma-separated list.
func TestIsSkippedBucket(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		skipBuckets string
		want        bool
	}{
		{"exact match", "hermes", "hermes", true},
		{"case-insensitive bucket", "Hermes", "hermes", true},
		{"case-insensitive config", "hermes", "Hermes", true},
		{"all caps", "HERMES", "hermes", true},
		{"no match", "my-bucket", "hermes", false},
		{"empty bucket", "", "hermes", false},
		{"disabled (empty config)", "hermes", "", false},
		{"list member with spaces", "audit", "hermes, audit , logs", true},
		{"list non-member", "data", "hermes,audit,logs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSkippedBucket(tt.bucket, tt.skipBuckets))
		})
	}
}

// scopedEntry builds an ops-log entry carrying a Keystone project domain.
func scopedEntry(domainID, domainName string) *S3OperationLog {
	return &S3OperationLog{
		KeystoneScope: &KeystoneScope{
			Project: KeystoneProject{
				Domain: KeystoneDomain{ID: domainID, Name: domainName},
			},
		},
	}
}

// TestMatchesAny verifies the case-insensitive, comma-separated token matcher
// used by the domain filter (a domain matches by its ID or its name).
func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		list       string
		want       bool
	}{
		{"match by name", []string{"id-1", "btp_fp"}, "btp_fp", true},
		{"match by id", []string{"1186c5f4", "btp_fp"}, "1186c5f4", true},
		{"case-insensitive candidate", []string{"BTP_FP"}, "btp_fp", true},
		{"case-insensitive token", []string{"btp_fp"}, "BTP_FP", true},
		{"list member with spaces", []string{"btp_fp"}, "foo, btp_fp , bar", true},
		{"no match", []string{"other"}, "btp_fp,foo", false},
		{"empty list", []string{"btp_fp"}, "", false},
		{"empty candidates", []string{"", ""}, "btp_fp", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesAny(tt.candidates, tt.list))
		})
	}
}

// TestIsDomainAudited verifies the domain scoping gate: deny wins over allow, a
// non-empty allow list restricts to its members (by domain ID or name), and an
// entry without a Keystone scope fails a non-empty allow list but passes when
// only a deny list (or neither) is set.
func TestIsDomainAudited(t *testing.T) {
	const (
		id   = "1186c5f4f6bc4977bc3446b9a0acdd0e"
		name = "btp_fp"
	)
	tests := []struct {
		name  string
		entry *S3OperationLog
		allow string
		deny  string
		want  bool
	}{
		{"no lists audits all", scopedEntry(id, name), "", "", true},
		{"allow by id", scopedEntry(id, name), id, "", true},
		{"allow by name", scopedEntry(id, name), name, "", true},
		{"multi-domain allow, member matches", scopedEntry(id, name), "dom-a," + name + ",dom-b", "", true},
		{"multi-domain allow, non-member drops", scopedEntry("other-id", "other"), "dom-a," + name + ",dom-b", "", false},
		{"multi-domain deny, member dropped", scopedEntry(id, name), "", "dom-a," + id + ",dom-b", false},
		{"allow excludes others", scopedEntry("other-id", "other"), name, "", false},
		{"deny by id", scopedEntry(id, name), "", id, false},
		{"deny by name", scopedEntry(id, name), "", name, false},
		{"deny beats allow", scopedEntry(id, name), name, name, false},
		{"deny only, other domain passes", scopedEntry("other-id", "other"), "", name, true},
		{"nil scope with allow list drops", &S3OperationLog{}, name, "", false},
		{"nil scope with deny only passes", &S3OperationLog{}, "", name, true},
		{"nil scope with no lists passes", &S3OperationLog{}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AuditSinkConfig{AllowDomains: tt.allow, DenyDomains: tt.deny}
			assert.Equal(t, tt.want, isDomainAudited(tt.entry, cfg))
		})
	}
}
