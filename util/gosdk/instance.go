package gosdk

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/util/logging"
	"github.com/linode/linodego"
)

// InstanceHandler contains helpers to handle Linode instances.
type InstanceHandler interface {
	CreateInstance(context.Context, linodego.InstanceCreateOptions) (*linodego.Instance, error)
}

// NewInstanceHandler constructs a Linode instance handler.
func NewInstanceHandler(client *Client, log logr.Logger, createTimeout time.Duration) InstanceHandler {
	return &instanceHandler{
		client:        client,
		createTimeout: int(createTimeout.Round(time.Second)),
		log:           log.WithName("instance-handler"),
	}
}

type instanceHandler struct {
	client        *Client
	createTimeout int
	log           logr.Logger
}

// CreateInstance creates an instance and wits for booting.
func (ih *instanceHandler) CreateInstance(ctx context.Context, insanceOptions linodego.InstanceCreateOptions) (*linodego.Instance, error) {
	instance, err := ih.client.CreateInstance(ctx, insanceOptions)
	if err != nil {
		return nil, logging.LogAndWrapError(ih.log, "instance creation failed", err, insanceOptions)
	}

	ih.log.V(3).Info("instance created", "ID", instance.ID)

	instance, err = ih.client.WaitForInstanceStatus(ctx, instance.ID, linodego.InstanceRunning, ih.createTimeout)
	if err != nil {
		return nil, logging.LogAndWrapError(ih.log, "wait for instance creation failed", err, insanceOptions)
	}

	ih.log.V(3).Info("instance running", "ID", instance.ID)

	return instance, nil
}
