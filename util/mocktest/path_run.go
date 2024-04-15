package mocktest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/onsi/ginkgo/v2"
)

// Run evaluates all declared mock client methods and assertions for the given test path.
func Run(ctx MockContext, p path) {
	if ctx.Context == nil {
		panic("MockContext requires Context")
	}
	if ctx.TestReporter == nil {
		panic("MockContext requires TestReporter, i.e. *testing.T, GinkgoT()")
	}

	for _, o := range p.once {
		evalOnce(ctx, o)
	}
	for _, c := range p.calls {
		evalFn(ctx, fn(c))
	}
	evalFn(ctx, fn(p.result))
}

func evalOnce(ctx MockContext, f *once) {
	if f.ran {
		return
	}

	switch tt := ctx.TestReporter.(type) {
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

func evalFn(ctx MockContext, f fn) {
	switch tt := ctx.TestReporter.(type) {
	case *testing.T:
		tt.Log(f.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(f.text)
	default:
		fmt.Println(f.text)
	}

	switch mockFunc := f.value.(type) {
	case func(MockContext):
		mockFunc(ctx)
	default:
		panic(errors.New("invalid function signature passed to Mock/Result"))
	}
}
