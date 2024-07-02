/*
Copyright 2024 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package linodeclient

import (
	"github.com/linode/linodego"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/linode/cluster-api-provider-linode/observability/wrappers"
)

const (
	NodebalancerIDParam = "nodebalancerID"
	ConfigIDParam       = "configID"
	DomainIDParam       = "domainID"
	DiskIDParam         = "diskID"
	NodeIDParam         = "nodeID"
	DomainRecordIDParam = "domainRecordID"
	LinodeIDParam       = "linodeID"
	KeyIDParam          = "keyID"
	VpcIDParam          = "vpcID"
	ImageIDParam        = "imageID"
	ClusterParam        = "cluster"
	LabelParam          = "label"
	RegionIDParam       = "regionID"
	TypeIDParam         = "typeID"
	SizeParam           = "size"
	RecordReqParam      = "recordReq"
)

func DefaultDecorator() func(span trace.Span, params, results map[string]interface{}) { //nolint:cyclop,gocognit  // TODO: refactor this
	return func(span trace.Span, params, results map[string]interface{}) {
		attr := []attribute.KeyValue{}

		if val, ok := wrappers.GetValue[int](params, NodebalancerIDParam); ok {
			attr = append(attr,
				attribute.Int("req.nodebalancer_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, ConfigIDParam); ok {
			attr = append(attr,
				attribute.Int("req.config_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, DomainIDParam); ok {
			attr = append(attr,
				attribute.Int("req.domain_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, DiskIDParam); ok {
			attr = append(attr,
				attribute.Int("req.disk_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, NodeIDParam); ok {
			attr = append(attr,
				attribute.Int("req.node_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, DomainRecordIDParam); ok {
			attr = append(attr,
				attribute.Int("req.domain_record_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, LinodeIDParam); ok {
			attr = append(attr,
				attribute.Int("req.linode_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, KeyIDParam); ok {
			attr = append(attr,
				attribute.Int("req.key_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, VpcIDParam); ok {
			attr = append(attr,
				attribute.Int("req.vpc_id", val),
			)
		}

		if val, ok := wrappers.GetValue[int](params, SizeParam); ok {
			attr = append(attr,
				attribute.Int("req.size", val),
			)
		}

		if val, ok := wrappers.GetValue[string](params, ImageIDParam); ok {
			attr = append(attr,
				attribute.String("req.image_id", val),
			)
		}

		if val, ok := wrappers.GetValue[string](params, ClusterParam); ok {
			attr = append(attr,
				attribute.String("req.cluster", val),
			)
		}

		if val, ok := wrappers.GetValue[string](params, LabelParam); ok {
			attr = append(attr,
				attribute.String("req.label", val),
			)
		}

		if val, ok := wrappers.GetValue[string](params, RegionIDParam); ok {
			attr = append(attr,
				attribute.String("req.region_id", val),
			)
		}

		if val, ok := wrappers.GetValue[string](params, TypeIDParam); ok {
			attr = append(attr,
				attribute.String("req.type_id", val),
			)
		}

		if val, ok := wrappers.GetValue[linodego.DomainRecordUpdateOptions](params, RecordReqParam); ok {
			attr = append(attr,
				attribute.String("req.domain_record_update_options.name", val.Name),
				attribute.String("req.domain_record_update_options.target", val.Target),
				attribute.String("req.domain_record_update_options.type", string(val.Type)),
				attribute.String("req.domain_record_update_options.protocol", wrappers.Optional(val.Protocol)),
				attribute.String("req.domain_record_update_options.service", wrappers.Optional(val.Service)),
				attribute.String("req.domain_record_update_options.tag", wrappers.Optional(val.Tag)),
				attribute.Int("req.domain_record_update_options.ttl_sec", val.TTLSec),
				attribute.Int("req.domain_record_update_options.port", wrappers.Optional(val.Port)),
				attribute.Int("req.domain_record_update_options.priority", wrappers.Optional(val.Priority)),
				attribute.Int("req.domain_record_update_options.weight", wrappers.Optional(val.Weight)),
			)
		}

		span.SetAttributes(attr...)
	}
}
