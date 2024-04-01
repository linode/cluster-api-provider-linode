package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
)

// AddBlockMoveAnnotation adds a block move annotation to the given object.
//
// obj: The object to add the annotation to.
// Returns: True if the annotation was added, false if it already existed.
func AddBlockMoveAnnotation(obj metav1.Object) bool {
	annotations := obj.GetAnnotations()
	if _, found := annotations[clusterctlv1.BlockMoveAnnotation]; found {
		return false
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[clusterctlv1.BlockMoveAnnotation] = "true"
	obj.SetAnnotations(annotations)
	return true
}

// RemoveBlockMoveAnnotation removes the clusterctlv1.BlockMoveAnnotation from the given object's annotations.
//
// obj: The metav1.Object from which the annotation needs to be removed.
// Return type: None.
func RemoveBlockMoveAnnotation(obj metav1.Object) {
	linodeClusterAnnotations := obj.GetAnnotations()
	delete(linodeClusterAnnotations, clusterctlv1.BlockMoveAnnotation)
	obj.SetAnnotations(linodeClusterAnnotations)
}
