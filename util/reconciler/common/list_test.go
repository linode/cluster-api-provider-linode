package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestObjectListKind(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()

	_, err := objectListKind(scheme, &corev1.Pod{})
	require.Error(t, err)

	utilruntime.Must(corev1.AddToScheme(scheme))

	podList, err := objectListKind(scheme, &corev1.Pod{})
	require.NoError(t, err)
	require.IsType(t, &corev1.PodList{}, podList)
}
