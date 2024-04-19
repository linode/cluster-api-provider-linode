package mocktest

import (
	"context"
	"testing"

	"github.com/onsi/ginkgo/v2"
)

// Run evaluates all declared mock client methods and assertions for the given test path.
func (p path) Run(ctx context.Context, mck Mock) {
	if mck.TestReporter == nil {
		panic("Mock requires TestReporter, i.e. *testing.T, GinkgoT()")
	}

	for _, o := range p.once {
		evalOnce(ctx, mck, o)
	}
	for _, c := range p.calls {
		evalFn(ctx, mck, fn(c))
	}
	evalFn(ctx, mck, fn(p.result))
}

func evalFn(ctx context.Context, mck Mock, fun fn) {
	switch tt := mck.TestReporter.(type) {
	case *testing.T:
		tt.Log(fun.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(fun.text)
	}

	fun.does(ctx, mck)
}

func evalOnce(ctx context.Context, mck Mock, fun *once) {
	if fun.ran {
		return
	}

	evalFn(ctx, mck, fn(*fun))

	fun.ran = true
}
