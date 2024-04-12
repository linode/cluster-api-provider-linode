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

// Run evaluates all declared mock client methods and assertions for the given code path.
// It receives any implementation of gomock.TestReporter (i.e. *testing.T, GinkgoT()).
// Its generic type is a mock client, which is usually a mock Linode client but may also be MockK8sClient.
// Both a Linode client and MockK8sClient may optionally be used by passing the MockK8sClient last.
func Run[T any](p path, t gomock.TestReporter, ctx context.Context, lc T, kc ...*mock.MockK8sClient) {
	for _, o := range p.once {
		evalOnce(t, ctx, o)
	}
	for _, c := range p.calls {
		evalCall[T](t, c, lc, kc...)
	}
	evalResult[T](t, ctx, p.result, lc, kc...)
}

func evalOnce(t gomock.TestReporter, ctx context.Context, f *once) {
	if f.ran {
		return
	}

	switch tt := t.(type) {
	case *testing.T:
		tt.Log(f.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(f.text)
	default:
		fmt.Println(f.text)
	}

	resultFn := f.value.(func(context.Context))
	resultFn(ctx)
	f.ran = true
}

// Evaluate the given mock client method.
func evalCall[T any](t gomock.TestReporter, f call, lc T, kc ...*mock.MockK8sClient) {
	switch tt := t.(type) {
	case *testing.T:
		tt.Log(f.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(f.text)
	default:
		fmt.Println(f.text)
	}

	switch mockFunc := f.value.(type) {
	// If the function expects MockK8sClient, then first check if T is MockK8sClient.
	// If not, then we expect MockK8sClient to have been passed in as the last argument.
	case func(*mock.MockK8sClient):
		// Use reflection to determine the type of T.
		// This is necessary since we can't do type assertion with generic types.
		if reflect.TypeOf(lc).Elem().Name() == "MockK8sClient" {
			mockFunc(reflect.ValueOf(lc).Interface().(*mock.MockK8sClient))
		} else if len(kc) == 0 {
			panic("called Mock with func(MockK8sClient) but without passing MockK8sClient to Run")
		} else {
			mockFunc(kc[0])
		}
	default:
		mockFunc.(func(T))(lc)
	}
}

// Evaluate the function for asserting results.
func evalResult[T any](t gomock.TestReporter, ctx context.Context, f result, lc T, kc ...*mock.MockK8sClient) {
	switch tt := t.(type) {
	case *testing.T:
		tt.Log(f.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(f.text)
	default:
		fmt.Println(f.text)
	}

	// If both a Linode client and a MockK8sClient are expected by the function, expect MockK8sClient to be present.
	if reflect.TypeOf(f.value).NumIn() > 2 {
		if len(kc) == 0 {
			panic("called Result with func expecting MockK8sClient but without passing MockK8sClient to Run")
		}

		resultFn := f.value.(func(context.Context, T, *mock.MockK8sClient))
		resultFn(ctx, lc, kc[0])
		return
	}

	switch resultFn := f.value.(type) {
	// If the function expects MockK8sClient, then first check if T is MockK8sClient.
	// If not, then we expect MockK8sClient to have been passed in as the last argument.
	case func(context.Context, *mock.MockK8sClient):
		// Use reflection to determine the type of T.
		// This is necessary since we can't do type assertion with generic types.
		if reflect.TypeOf(lc).Elem().Name() == "MockK8sClient" {
			resultFn(ctx, reflect.ValueOf(lc).Interface().(*mock.MockK8sClient))
		} else if len(kc) == 0 {
			panic("called Result with func(ctx, MockK8sClient) but without passing MockK8sClient to Run")
		} else {
			resultFn(ctx, kc[0])
		}
	default:
		resultFn.(func(context.Context, T))(ctx, lc)
	}
}
