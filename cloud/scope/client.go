package scope

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type crClient interface {
    Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error
}
