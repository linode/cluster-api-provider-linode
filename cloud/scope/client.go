package scope

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sReader interface {
    client.Reader
	client.Writer
}
