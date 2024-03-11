/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scope

import (
	"context"
	"reflect"
	"testing"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_validateVPCScopeParams(t *testing.T) {
	type args struct {
		params VPCScopeParams
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Valid VPCScopeParams",
			args: args{
				params: VPCScopeParams{
					LinodeVPC: &infrav1alpha1.LinodeVPC{},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid VPCScopeParams",
			args: args{
				params: VPCScopeParams{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateVPCScopeParams(tt.args.params); (err != nil) != tt.wantErr {
				t.Errorf("validateVPCScopeParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewVPCScope(t *testing.T) {
	type args struct {
		ctx    context.Context
		apiKey string
		params VPCScopeParams
	}
	tests := []struct {
		name    string
		args    args
		want    *VPCScope
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVPCScope(tt.args.ctx, tt.args.apiKey, tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVPCScope() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewVPCScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVPCScope_AddFinalizer(t *testing.T) {
	type fields struct {
		client       client.Client
		PatchHelper  *patch.Helper
		LinodeClient *linodego.Client
		LinodeVPC    *infrav1alpha1.LinodeVPC
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &VPCScope{
				client:       tt.fields.client,
				PatchHelper:  tt.fields.PatchHelper,
				LinodeClient: tt.fields.LinodeClient,
				LinodeVPC:    tt.fields.LinodeVPC,
			}
			if err := s.AddFinalizer(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("VPCScope.AddFinalizer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
