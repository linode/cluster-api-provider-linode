package testmock

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/onsi/ginkgo/v2"
	"go.uber.org/mock/gomock"
)

func (p path) Run(t gomock.TestReporter, ctx context.Context, lc any, kc ...*mock.MockK8sClient) {
	switch lc.(type) {
	case *mock.MockK8sClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockK8sClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockK8sClient](t, evt, lc, kc...)
			}
		}
	case *mock.MockLinodeMachineClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockLinodeMachineClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockLinodeMachineClient](t, evt, lc, kc...)
			}
		}
	case *mock.MockLinodeInstanceClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockLinodeInstanceClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockLinodeInstanceClient](t, evt, lc, kc...)
			}
		}
	case *mock.MockLinodeVPCClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockLinodeVPCClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockLinodeVPCClient](t, evt, lc, kc...)
			}
		}
	case *mock.MockLinodeNodeBalancerClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockLinodeNodeBalancerClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockLinodeNodeBalancerClient](t, evt, lc, kc...)
			}
		}
	case *mock.MockLinodeObjectStorageClient:
		for _, evt := range p.events {
			if evt.isResult {
				mockResult[*mock.MockLinodeObjectStorageClient](t, ctx, evt, lc, kc...)
			} else {
				mockCall[*mock.MockLinodeObjectStorageClient](t, evt, lc, kc...)
			}
		}
	default:
		panic("passed unknown client type to Run")
	}
}

func mockCall[T any](t gomock.TestReporter, evt event, lc any, kc ...*mock.MockK8sClient) {
	switch tt := t.(type) {
	case *testing.T:
		tt.Log(evt.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(evt.text)
	default:
		fmt.Println(evt.text)
	}

	switch mockFunc := evt.value.(type) {
	case func(*mock.MockK8sClient):
		if asKC, ok := lc.(*mock.MockK8sClient); ok {
			mockFunc(asKC)
		} else if len(kc) == 0 {
			panic("called Mock with func(MockK8sClient) but without passing MockK8sClient to Run")
		} else {
			mockFunc(kc[0])
		}
	default:
		mockFunc.(func(T))(lc.(T))
	}
}

func mockResult[T any](t gomock.TestReporter, ctx context.Context, evt event, lc any, kc ...*mock.MockK8sClient) {
	switch tt := t.(type) {
	case *testing.T:
		tt.Log(evt.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(evt.text)
	default:
		fmt.Println(evt.text)
	}

	if reflect.TypeOf(evt.value).NumIn() > 2 {
		if len(kc) == 0 {
			panic("called Result with func(ctx, LinodeClient, MockK8sClient) but without passing MockK8sClient to Run")
		}

		mockFunc := evt.value.(func(context.Context, T, *mock.MockK8sClient))
		mockFunc(ctx, lc.(T), kc[0])
		return
	}

	switch mockFunc := evt.value.(type) {
	case func(context.Context, *mock.MockK8sClient):
		if asKC, ok := lc.(*mock.MockK8sClient); ok {
			mockFunc(ctx, asKC)
		} else if len(kc) == 0 {
			panic("called Result with func(ctx, MockK8sClient) but without passing MockK8sClient to Run")
		} else {
			mockFunc(ctx, kc[0])
		}
	default:
		mockFunc.(func(context.Context, T))(ctx, lc.(T))
	}
}
