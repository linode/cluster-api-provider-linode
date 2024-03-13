package services

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/linodego"
)

func TestCreateNodeBalancer(t *testing.T) {
	type args struct {
		ctx          context.Context
		clusterScope *scope.ClusterScope
		logger       logr.Logger
	}
	tests := []struct {
		name    string
		args    args
		want    *linodego.NodeBalancer
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateNodeBalancer(tt.args.ctx, tt.args.clusterScope, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNodeBalancer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateNodeBalancer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateNodeBalancerConfig(t *testing.T) {
	type args struct {
		ctx          context.Context
		clusterScope *scope.ClusterScope
		logger       logr.Logger
	}
	tests := []struct {
		name    string
		args    args
		want    *linodego.NodeBalancerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateNodeBalancerConfig(tt.args.ctx, tt.args.clusterScope, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNodeBalancerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateNodeBalancerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddNodeToNB(t *testing.T) {
	type args struct {
		ctx          context.Context
		logger       logr.Logger
		machineScope *scope.MachineScope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddNodeToNB(tt.args.ctx, tt.args.logger, tt.args.machineScope); (err != nil) != tt.wantErr {
				t.Errorf("AddNodeToNB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteNodeFromNB(t *testing.T) {
	type args struct {
		ctx          context.Context
		logger       logr.Logger
		machineScope *scope.MachineScope
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteNodeFromNB(tt.args.ctx, tt.args.logger, tt.args.machineScope); (err != nil) != tt.wantErr {
				t.Errorf("DeleteNodeFromNB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
