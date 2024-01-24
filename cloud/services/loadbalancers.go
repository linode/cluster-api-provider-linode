package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/linodego"
)

var (
	defaultLBPort = 6443
)

// CreateNodeBalancer creates a new NodeBalancer if one doesn't exist
func CreateNodeBalancer(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancer, error) {
	var linodeNBs []linodego.NodeBalancer
	var linodeNB *linodego.NodeBalancer
	NBLabel := fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name)
	clusterUID := string(clusterScope.LinodeCluster.UID)
	tags := []string{string(clusterScope.LinodeCluster.UID)}
	filter := map[string]string{
		"label": NBLabel,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	logger.Info("Creating NodeBalancer")
	if linodeNBs, err = clusterScope.LinodeClient.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(rawFilter))); err != nil {
		logger.Info("Failed to list NodeBalancers", "error", err.Error())

		return nil, err
	}

	switch len(linodeNBs) {
	case 1:
		logger.Info(fmt.Sprintf("NodeBalancer %s already exists", *linodeNBs[0].Label))
		if !slices.Contains(linodeNBs[0].Tags, clusterUID) {
			err = errors.New("NodeBalancer conflict")
			logger.Error(err, fmt.Sprintf("NodeBalancer %s is not associated with cluster UID %s. Owner cluster is %s", *linodeNBs[0].Label, clusterUID, linodeNBs[0].Tags[0]))

			return nil, err
		}
		linodeNB = &linodeNBs[0]
	case 0:
		logger.Info(fmt.Sprintf("Creating NodeBalancer %s-api-server", clusterScope.LinodeCluster.Name))
		createConfig := linodego.NodeBalancerCreateOptions{
			Label:              util.Pointer(fmt.Sprintf("%s-api-server", clusterScope.LinodeCluster.Name)),
			Region:             clusterScope.LinodeCluster.Spec.Region,
			ClientConnThrottle: nil,
			Tags:               tags,
		}

		if linodeNB, err = clusterScope.LinodeClient.CreateNodeBalancer(ctx, createConfig); err != nil {
			logger.Info("Failed to create Linode NodeBalancer", "error", err.Error())

			// Already exists is not an error
			apiErr := linodego.Error{}
			if errors.As(err, &apiErr) && apiErr.Code != http.StatusFound {
				return nil, err
			}

			err = nil

			if linodeNB != nil {
				logger.Info("Linode NodeBalancer already exists", "existing", linodeNB.Label)
			}
		}

	default:
		err = errors.New("multiple NodeBalancers")

		logger.Error(err, "Panic! Multiple NodeBalancers found. This might be a concurrency issue in the controller!!!", "filters", string(rawFilter))

		return nil, err
	}

	if linodeNB == nil {
		err = errors.New("missing NodeBalancer")

		logger.Error(err, "Panic! Failed to create NodeBalancer")

		return nil, err
	}

	return linodeNB, nil
}

// CreateNodeBalancerConfig creates NodeBalancer config if it does not exist
func CreateNodeBalancerConfig(ctx context.Context, clusterScope *scope.ClusterScope, logger logr.Logger) (*linodego.NodeBalancerConfig, error) {
	var linodeNBConfigs []linodego.NodeBalancerConfig
	var linodeNBConfig *linodego.NodeBalancerConfig
	var err error

	if linodeNBConfigs, err = clusterScope.LinodeClient.ListNodeBalancerConfigs(ctx, clusterScope.LinodeCluster.Spec.Network.NodeBalancerID, linodego.NewListOptions(1, "")); err != nil {
		logger.Info("Failed to list NodeBalancer Configs", "error", err.Error())

		return nil, err
	}
	lbPort := defaultLBPort
	if clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort != 0 {
		lbPort = clusterScope.LinodeCluster.Spec.Network.LoadBalancerPort
	}
	switch len(linodeNBConfigs) {
	case 1:
		logger.Info("NodeBalancer ", strconv.Itoa(linodeNBConfigs[0].ID), " already exists")
		linodeNBConfig = &linodeNBConfigs[0]

	case 0:
		createConfig := linodego.NodeBalancerConfigCreateOptions{
			Port:      lbPort,
			Protocol:  linodego.ProtocolTCP,
			Algorithm: linodego.AlgorithmRoundRobin,
			Check:     linodego.CheckConnection,
		}

		if linodeNBConfig, err = clusterScope.LinodeClient.CreateNodeBalancerConfig(ctx, clusterScope.LinodeCluster.Spec.Network.NodeBalancerID, createConfig); err != nil {
			logger.Info("Failed to create Linode NodeBalancer config", "error", err.Error())

			return nil, err
		}

	default:
		err = errors.New("multiple NodeBalancer Configs")

		logger.Error(err, "Panic! Multiple NodeBalancer Configs found. This might be a concurrency issue in the controller!!!")

		return nil, err
	}

	return linodeNBConfig, nil
}
