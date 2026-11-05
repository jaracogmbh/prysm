// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// collectCounterMetrics is a test helper that collects only counter metrics matching the counter desc.
func (c *sliCollector) collectCounterMetrics() []*dto.Metric {
	ch := make(chan prometheus.Metric, 1000)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	var results []*dto.Metric
	for m := range ch {
		d := &dto.Metric{}
		if err := m.Write(d); err == nil {
			if d.GetCounter() != nil {
				results = append(results, d)
			}
		}
	}
	return results
}

// findCounterByLabels is a test helper to find a specific counter metric from collected output.
func findCounterByLabels(metrics []*dto.Metric, tenant, protocol, operation, statusClass string) *dto.Metric {
	for _, m := range metrics {
		labels := map[string]string{}
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		if labels["tenant"] == tenant &&
			labels["protocol"] == protocol &&
			labels["operation"] == operation &&
			labels["status_class"] == statusClass {
			return m
		}
	}
	return nil
}

// TestObserveSLI_TenantFallbackForLocalAuth verifies that on clusters without
// Keystone (pure S3 / Local auth, e.g. Rook), where extractUserAndTenant
// resolves every request to tenant="none", observeSLI falls back to the S3
// user as the tenant so the SLI metrics are actually populated. Keystone-style
// "user$tenant" users must keep their real tenant, and requests whose
// operation is not SLI-classifiable must still be dropped.
func TestObserveSLI_TenantFallbackForLocalAuth(t *testing.T) {
	original := globalSLICollector
	defer func() { globalSLICollector = original }()

	globalSLICollector = newSLICollector(SLICollectorConfig{
		StaleTTL:           time.Hour,
		ReapInterval:       time.Minute,
		Region:             "qa-de-1",
		ErrorCodeAllowlist: []string{"InternalError", "ServiceUnavailable"},
	})

	// Local auth (Rook): user has no "$tenant" suffix → tenant "none".
	observeSLI(S3OperationLog{
		Bucket:     "ceph-loki",
		User:       "my-user",
		Operation:  "list_bucket",
		URI:        "GET /ceph-loki?list-type=2 HTTP/1.1",
		HTTPStatus: "200",
		TotalTime:  12,
	}, "none")

	// Keystone-style user keeps its real tenant.
	observeSLI(S3OperationLog{
		Bucket:     "openstack-bucket",
		User:       "alice$project-x",
		Operation:  "get_obj",
		URI:        "GET /openstack-bucket/key HTTP/1.1",
		HTTPStatus: "200",
		TotalTime:  5,
	}, "project-x")

	// Unclassifiable operation is dropped even with the fallback.
	observeSLI(S3OperationLog{
		Bucket:     "ceph-loki",
		User:       "my-user",
		Operation:  "create_bucket",
		URI:        "PUT /ceph-loki HTTP/1.1",
		HTTPStatus: "200",
		TotalTime:  3,
	}, "none")

	counters := globalSLICollector.collectCounterMetrics()

	// Local-auth request recorded under the S3 user as tenant.
	m := findCounterByLabels(counters, "my-user", "s3", "list", "2xx")
	if m == nil {
		t.Fatalf("expected radosgw_request_total{tenant=my-user,protocol=s3,operation=list,status_class=2xx} to exist; got %d counters", len(counters))
	}
	if got := m.GetCounter().GetValue(); got != 1 {
		t.Errorf("local-auth fallback counter = %f, want 1", got)
	}

	// Keystone request keeps its tenant.
	if findCounterByLabels(counters, "project-x", "s3", "get", "2xx") == nil {
		t.Errorf("expected radosgw_request_total{tenant=project-x,protocol=s3,operation=get,status_class=2xx} to exist")
	}

	// Unclassifiable operation must not create a series for my-user.
	if findCounterByLabels(counters, "my-user", "s3", "", "2xx") != nil {
		t.Errorf("unclassifiable operation should not produce a series")
	}
	for _, m := range counters {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "tenant" && lp.GetValue() == "none" {
				t.Errorf("tenant=none series must never be emitted; got one for operation %s", m.GetLabel())
			}
		}
	}
}
