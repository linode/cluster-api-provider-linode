package mocktest

import (
	"context"
	"testing"

	"github.com/onsi/ginkgo/v2"
)

// Run evaluates all declared mock client methods and assertions for the given test path.
func (p path) Run(ctx context.Context, m Mock) {
	if m.TestReporter == nil {
		panic("Mock requires TestReporter, i.e. *testing.T, GinkgoT()")
	}

	for _, o := range p.once {
		evalOnce(ctx, m, o)
	}
	for _, c := range p.calls {
		evalFn(ctx, m, fn(c))
	}
	evalFn(ctx, m, fn(p.result))
}

func evalFn(ctx context.Context, m Mock, fun fn) {
	switch tt := m.TestReporter.(type) {
	case *testing.T:
		tt.Log(fun.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(fun.text)
	}

	fun.does(ctx, m)
}

func evalOnce(ctx context.Context, m Mock, fun *once) {
	if fun.ran {
		return
	}

	switch tt := m.TestReporter.(type) {
	case *testing.T:
		tt.Log(fun.text)
	case ginkgo.GinkgoTInterface:
		ginkgo.By(fun.text)
	}

	fun.does(ctx)
	fun.ran = true
}
