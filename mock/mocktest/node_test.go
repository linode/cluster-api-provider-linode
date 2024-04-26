package mocktest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOneOfWithoutNodes(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		OneOf()
	})
}

func TestPathWithoutNodes(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		Path()
	})
}
