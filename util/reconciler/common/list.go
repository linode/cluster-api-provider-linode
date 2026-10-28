package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// objectListKind returns a new list type corresponding to the given Kubernetes object.
// It uses the object's GroupVersionKind to determine the appropriate list type.
func objectListKind(scheme *runtime.Scheme, obj client.Object) (client.ObjectList, error) {
	gvk, err := objectKind(obj, scheme)
	if err != nil {
		return nil, err
	}

	return listTypeFromSingularGVK(scheme, gvk)
}

// listTypeFromSingularGVK returns a new list type corresponding to the given singular GroupVersionKind.
func listTypeFromSingularGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) (client.ObjectList, error) {
	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	list, err := scheme.New(listGVK)
	if err != nil {
		return nil, err
	}

	listed, ok := list.(client.ObjectList)
	if !ok {
		return nil, fmt.Errorf("runtime.Object %T has no client.ObjectList", list)
	}

	return listed, nil
}

// objectKind returns the GroupVersionKind of a Kubernetes object.
// It first checks if the GVK is set on the object itself. If not, it attempts to get the GVK from the scheme.
// It then sets the GVK on the object.
func objectKind(obj client.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if !gvk.Empty() {
		return gvk, nil
	}

	// If the GVK is not set, attempt to get it from the scheme.
	var err error
	gvk, err = apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return gvk, nil
}
