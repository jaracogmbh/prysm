// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package opslog

import (
	"testing"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestPublishRequestCounters_PerBucketLabels(t *testing.T) {
	totalRequestsPerBucketCounter.Reset()

	diffMetrics := NewMetrics()
	diffMetrics.RequestsByBucket.Store("alice$tenant-a|bucket-a|GET|200", newUint64(7))

	publishRequestCounters(diffMetrics, OpsLogConfig{
		PodName: "rgw-a-0",
		MetricsConfig: MetricsConfig{
			TrackRequestsPerBucket: true,
		},
	})

	metric := &io_prometheus_client.Metric{}
	err := totalRequestsPerBucketCounter.WithLabelValues("rgw-a-0", "tenant-a", "bucket-a", "GET", "200").Write(metric)
	assert.NoError(t, err)
	assert.Equal(t, 7.0, metric.GetCounter().GetValue())
}
