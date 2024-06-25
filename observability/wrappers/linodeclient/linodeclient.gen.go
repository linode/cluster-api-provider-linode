// Code generated by gowrap. DO NOT EDIT.
// template: ../../../hack/templates/opentelemetry.go.gotpl
// gowrap: http://github.com/hexdigest/gowrap

package linodeclient

//go:generate gowrap gen -p github.com/linode/cluster-api-provider-linode/clients -i LinodeClient -t ../../../hack/templates/opentelemetry.go.gotpl -o linodeclient.gen.go -l ""

import (
	"context"

	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/observability/tracing"
	"github.com/linode/linodego"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// LinodeClientWithTracing implements clients.LinodeClient interface instrumented with opentracing spans
type LinodeClientWithTracing struct {
	clients.LinodeClient
	_spanDecorator func(span trace.Span, params, results map[string]interface{})
}

// NewLinodeClientWithTracing returns LinodeClientWithTracing
func NewLinodeClientWithTracing(base clients.LinodeClient, spanDecorator ...func(span trace.Span, params, results map[string]interface{})) LinodeClientWithTracing {
	d := LinodeClientWithTracing{
		LinodeClient: base,
	}

	if len(spanDecorator) > 0 && spanDecorator[0] != nil {
		d._spanDecorator = spanDecorator[0]
	}

	return d
}

// BootInstance implements clients.LinodeClient
func (_d LinodeClientWithTracing) BootInstance(ctx context.Context, linodeID int, configID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.BootInstance")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"configID": configID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.BootInstance(ctx, linodeID, configID)
}

// CreateDomainRecord implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateDomainRecord(ctx context.Context, domainID int, recordReq linodego.DomainRecordCreateOptions) (dp1 *linodego.DomainRecord, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateDomainRecord")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":       ctx,
				"domainID":  domainID,
				"recordReq": recordReq}, map[string]interface{}{
				"dp1": dp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateDomainRecord(ctx, domainID, recordReq)
}

// CreateInstance implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateInstance(ctx context.Context, opts linodego.InstanceCreateOptions) (ip1 *linodego.Instance, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateInstance")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateInstance(ctx, opts)
}

// CreateInstanceDisk implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateInstanceDisk(ctx context.Context, linodeID int, opts linodego.InstanceDiskCreateOptions) (ip1 *linodego.InstanceDisk, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateInstanceDisk")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"opts":     opts}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateInstanceDisk(ctx, linodeID, opts)
}

// CreateNodeBalancer implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateNodeBalancer(ctx context.Context, opts linodego.NodeBalancerCreateOptions) (np1 *linodego.NodeBalancer, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateNodeBalancer")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"np1": np1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateNodeBalancer(ctx, opts)
}

// CreateNodeBalancerConfig implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateNodeBalancerConfig(ctx context.Context, nodebalancerID int, opts linodego.NodeBalancerConfigCreateOptions) (np1 *linodego.NodeBalancerConfig, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateNodeBalancerConfig")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"nodebalancerID": nodebalancerID,
				"opts":           opts}, map[string]interface{}{
				"np1": np1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateNodeBalancerConfig(ctx, nodebalancerID, opts)
}

// CreateNodeBalancerNode implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, opts linodego.NodeBalancerNodeCreateOptions) (np1 *linodego.NodeBalancerNode, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateNodeBalancerNode")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"nodebalancerID": nodebalancerID,
				"configID":       configID,
				"opts":           opts}, map[string]interface{}{
				"np1": np1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateNodeBalancerNode(ctx, nodebalancerID, configID, opts)
}

// CreateObjectStorageBucket implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (op1 *linodego.ObjectStorageBucket, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateObjectStorageBucket")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"op1": op1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateObjectStorageBucket(ctx, opts)
}

// CreateObjectStorageKey implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (op1 *linodego.ObjectStorageKey, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateObjectStorageKey")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"op1": op1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateObjectStorageKey(ctx, opts)
}

// CreateStackscript implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateStackscript(ctx context.Context, opts linodego.StackscriptCreateOptions) (sp1 *linodego.Stackscript, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateStackscript")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"sp1": sp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateStackscript(ctx, opts)
}

// CreateVPC implements clients.LinodeClient
func (_d LinodeClientWithTracing) CreateVPC(ctx context.Context, opts linodego.VPCCreateOptions) (vp1 *linodego.VPC, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.CreateVPC")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"vp1": vp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.CreateVPC(ctx, opts)
}

// DeleteDomainRecord implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteDomainRecord(ctx context.Context, domainID int, domainRecordID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteDomainRecord")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"domainID":       domainID,
				"domainRecordID": domainRecordID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteDomainRecord(ctx, domainID, domainRecordID)
}

// DeleteInstance implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteInstance(ctx context.Context, linodeID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteInstance")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteInstance(ctx, linodeID)
}

// DeleteNodeBalancer implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteNodeBalancer(ctx context.Context, nodebalancerID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteNodeBalancer")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"nodebalancerID": nodebalancerID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteNodeBalancer(ctx, nodebalancerID)
}

// DeleteNodeBalancerNode implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, nodeID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteNodeBalancerNode")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"nodebalancerID": nodebalancerID,
				"configID":       configID,
				"nodeID":         nodeID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteNodeBalancerNode(ctx, nodebalancerID, configID, nodeID)
}

// DeleteObjectStorageKey implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteObjectStorageKey(ctx context.Context, keyID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteObjectStorageKey")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":   ctx,
				"keyID": keyID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteObjectStorageKey(ctx, keyID)
}

// DeleteVPC implements clients.LinodeClient
func (_d LinodeClientWithTracing) DeleteVPC(ctx context.Context, vpcID int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.DeleteVPC")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":   ctx,
				"vpcID": vpcID}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.DeleteVPC(ctx, vpcID)
}

// GetImage implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetImage(ctx context.Context, imageID string) (ip1 *linodego.Image, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetImage")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":     ctx,
				"imageID": imageID}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetImage(ctx, imageID)
}

// GetInstance implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetInstance(ctx context.Context, linodeID int) (ip1 *linodego.Instance, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetInstance")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetInstance(ctx, linodeID)
}

// GetInstanceDisk implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetInstanceDisk(ctx context.Context, linodeID int, diskID int) (ip1 *linodego.InstanceDisk, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetInstanceDisk")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"diskID":   diskID}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetInstanceDisk(ctx, linodeID, diskID)
}

// GetInstanceIPAddresses implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetInstanceIPAddresses(ctx context.Context, linodeID int) (ip1 *linodego.InstanceIPAddressResponse, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetInstanceIPAddresses")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetInstanceIPAddresses(ctx, linodeID)
}

// GetObjectStorageBucket implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetObjectStorageBucket(ctx context.Context, cluster string, label string) (op1 *linodego.ObjectStorageBucket, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetObjectStorageBucket")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":     ctx,
				"cluster": cluster,
				"label":   label}, map[string]interface{}{
				"op1": op1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetObjectStorageBucket(ctx, cluster, label)
}

// GetObjectStorageKey implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetObjectStorageKey(ctx context.Context, keyID int) (op1 *linodego.ObjectStorageKey, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetObjectStorageKey")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":   ctx,
				"keyID": keyID}, map[string]interface{}{
				"op1": op1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetObjectStorageKey(ctx, keyID)
}

// GetRegion implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetRegion(ctx context.Context, regionID string) (rp1 *linodego.Region, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetRegion")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"regionID": regionID}, map[string]interface{}{
				"rp1": rp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetRegion(ctx, regionID)
}

// GetType implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetType(ctx context.Context, typeID string) (lp1 *linodego.LinodeType, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetType")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":    ctx,
				"typeID": typeID}, map[string]interface{}{
				"lp1": lp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetType(ctx, typeID)
}

// GetVPC implements clients.LinodeClient
func (_d LinodeClientWithTracing) GetVPC(ctx context.Context, vpcID int) (vp1 *linodego.VPC, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.GetVPC")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":   ctx,
				"vpcID": vpcID}, map[string]interface{}{
				"vp1": vp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.GetVPC(ctx, vpcID)
}

// ListDomainRecords implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListDomainRecords(ctx context.Context, domainID int, opts *linodego.ListOptions) (da1 []linodego.DomainRecord, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListDomainRecords")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"domainID": domainID,
				"opts":     opts}, map[string]interface{}{
				"da1": da1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListDomainRecords(ctx, domainID, opts)
}

// ListDomains implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListDomains(ctx context.Context, opts *linodego.ListOptions) (da1 []linodego.Domain, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListDomains")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"da1": da1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListDomains(ctx, opts)
}

// ListInstanceConfigs implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListInstanceConfigs(ctx context.Context, linodeID int, opts *linodego.ListOptions) (ia1 []linodego.InstanceConfig, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListInstanceConfigs")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"opts":     opts}, map[string]interface{}{
				"ia1": ia1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListInstanceConfigs(ctx, linodeID, opts)
}

// ListInstances implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListInstances(ctx context.Context, opts *linodego.ListOptions) (ia1 []linodego.Instance, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListInstances")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"ia1": ia1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListInstances(ctx, opts)
}

// ListNodeBalancers implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListNodeBalancers(ctx context.Context, opts *linodego.ListOptions) (na1 []linodego.NodeBalancer, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListNodeBalancers")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"na1": na1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListNodeBalancers(ctx, opts)
}

// ListStackscripts implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListStackscripts(ctx context.Context, opts *linodego.ListOptions) (sa1 []linodego.Stackscript, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListStackscripts")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"sa1": sa1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListStackscripts(ctx, opts)
}

// ListVPCs implements clients.LinodeClient
func (_d LinodeClientWithTracing) ListVPCs(ctx context.Context, opts *linodego.ListOptions) (va1 []linodego.VPC, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ListVPCs")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":  ctx,
				"opts": opts}, map[string]interface{}{
				"va1": va1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ListVPCs(ctx, opts)
}

// ResizeInstanceDisk implements clients.LinodeClient
func (_d LinodeClientWithTracing) ResizeInstanceDisk(ctx context.Context, linodeID int, diskID int, size int) (err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.ResizeInstanceDisk")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"diskID":   diskID,
				"size":     size}, map[string]interface{}{
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.ResizeInstanceDisk(ctx, linodeID, diskID, size)
}

// UpdateDomainRecord implements clients.LinodeClient
func (_d LinodeClientWithTracing) UpdateDomainRecord(ctx context.Context, domainID int, domainRecordID int, recordReq linodego.DomainRecordUpdateOptions) (dp1 *linodego.DomainRecord, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.UpdateDomainRecord")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":            ctx,
				"domainID":       domainID,
				"domainRecordID": domainRecordID,
				"recordReq":      recordReq}, map[string]interface{}{
				"dp1": dp1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.UpdateDomainRecord(ctx, domainID, domainRecordID, recordReq)
}

// UpdateInstanceConfig implements clients.LinodeClient
func (_d LinodeClientWithTracing) UpdateInstanceConfig(ctx context.Context, linodeID int, configID int, opts linodego.InstanceConfigUpdateOptions) (ip1 *linodego.InstanceConfig, err error) {
	ctx, _span := tracing.Start(ctx, "clients.LinodeClient.UpdateInstanceConfig")
	defer func() {
		if _d._spanDecorator != nil {
			_d._spanDecorator(_span, map[string]interface{}{
				"ctx":      ctx,
				"linodeID": linodeID,
				"configID": configID,
				"opts":     opts}, map[string]interface{}{
				"ip1": ip1,
				"err": err})
		} else if err != nil {
			_span.RecordError(err)
			_span.SetAttributes(
				attribute.String("event", "error"),
				attribute.String("message", err.Error()),
			)
		}

		_span.End()
	}()
	return _d.LinodeClient.UpdateInstanceConfig(ctx, linodeID, configID, opts)
}
