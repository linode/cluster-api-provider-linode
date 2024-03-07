package scope

import (
	"context"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sClient interface {
	client.Client
}

type PatchHelper interface {
	Patch(ctx context.Context, obj client.Object, opts ...patch.Option) error
}
