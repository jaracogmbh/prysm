// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and prysm contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/cobaltcore-dev/prysm/pkg/producers/bucketnotify"
	"github.com/cobaltcore-dev/prysm/pkg/producers/resourceusage"
)

// ProducerHandlerFunc is a function that starts a producer from config.
type ProducerHandlerFunc func(producer ProducerConfig, globalConfig GlobalConfig)

// producerHandlers holds registered producer handlers (used for build-tag-gated producers).
var producerHandlers = map[string]ProducerHandlerFunc{}

// registerProducerHandler registers a handler for a producer type.
// Called from init() in build-tag-gated files.
func registerProducerHandler(producerType string, handler ProducerHandlerFunc) {
	producerHandlers[producerType] = handler
}

func StartProducers(producer ProducerConfig, globalConfig GlobalConfig, wg *sync.WaitGroup) {
	defer wg.Done()

	natsURL := GetStringSetting(producer.Settings, "nats_url", globalConfig.NatsURL)
	// adminURL := GetStringSetting(producer.Settings, "admin_url", globalConfig.AdminURL)
	// accessKey := GetStringSetting(producer.Settings, "access_key", globalConfig.AccessKey)
	// secretKey := GetStringSetting(producer.Settings, "secret_key", globalConfig.SecretKey)
	nodeName := GetStringSetting(producer.Settings, "node_name", globalConfig.NodeName)
	instanceID := GetStringSetting(producer.Settings, "instance_id", globalConfig.InstanceID)

	switch producer.Type {
	case "bucket_notify":
		natsSubject := GetStringSetting(producer.Settings, "nats_subject", "rgw.buckets.notify")
		endpointPort := GetIntSetting(producer.Settings, "endpoint_port", 8080)
		settings := bucketnotify.BucketNotifyConfig{
			EndpointPort: endpointPort,
			NatsURL:      natsURL,
			NatsSubject:  natsSubject,
			UseNats:      natsURL != "",
		}
		log.Info().Msg("--- bucket notify ---")
		bucketnotify.StartBucketNotifyServer(settings)
	case "disk_health_metrics":
		log.Info().Msg("--- disk health metrics ---")
	case "kernel_metrics":
		log.Info().Msg("--- kernel metrics ---")
	case "resource_usage":
		natsSubject := GetStringSetting(producer.Settings, "nats_subject", "rgw.buckets.notify")
		prometheus := GetBoolSetting(producer.Settings, "prometheus", false)
		prometheusPort := GetIntSetting(producer.Settings, "endpoint_port", 8080)
		interval := GetIntSetting(producer.Settings, "interval", 30)
		disks := GetStringSliceSetting(producer.Settings, "disks", []string{"sda", "sdb"})
		settings := resourceusage.ResourceUsageConfig{
			NatsURL:        natsURL,
			NatsSubject:    natsSubject,
			UseNats:        natsURL != "",
			Prometheus:     prometheus,
			PrometheusPort: prometheusPort,
			Interval:       interval,
			Disks:          disks,
			NodeName:       nodeName,
			InstanceID:     instanceID,
		}
		log.Info().Msg("--- resource usage ---")
		resourceusage.StartMonitoring(settings)
	// case "quota_usage":
	// 	natsSubject := GetStringSetting(producer.Settings, "nats_subject", "rgw.buckets.notify")
	// 	interval := GetIntSetting(producer.Settings, "monitor_interval", 30)
	// 	quotaUsagePercent := GetFloat64Setting(producer.Settings, "quota_usage_percent", 80)
	// 	settings := quotausagemonitor.QuotaUsageMonitorConfig{
	// 		NatsURL:           natsURL,
	// 		NatsSubject:       natsSubject,
	// 		UseNats:           natsURL != "",
	// 		AdminURL:          adminURL,
	// 		AccessKey:         accessKey,
	// 		SecretKey:         secretKey,
	// 		Interval:          interval,
	// 		NodeName:          nodeName,
	// 		InstanceID:        instanceID,
	// 		QuotaUsagePercent: quotaUsagePercent,
	// 	}
	// 	quotausagemonitor.StartMonitoring(settings)
	default:
		if handler, ok := producerHandlers[producer.Type]; ok {
			handler(producer, globalConfig)
		} else {
			log.Warn().Msgf("unknown producer type: %s", producer.Type)
		}
	}
}
