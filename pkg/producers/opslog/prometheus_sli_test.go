// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
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
