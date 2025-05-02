/*
Copyright 2024 Akamai Technologies, Inc.

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

package wrappers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptional(t *testing.T) {
	t.Parallel()

	t.Run("int", func(t *testing.T) {
		t.Parallel()

		val := 1

		ret := Optional[int](&val)
		assert.Equal(t, val, ret)

		ret = Optional[int](nil)
		assert.Equal(t, 0, ret)
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		val := "foo"

		ret := Optional[string](&val)
		assert.Equal(t, val, ret)

		ret = Optional[string](nil)
		assert.Empty(t, ret)
	})

	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		val := []string{"foo", "bar"}

		ret := Optional[[]string](&val)
		assert.Equal(t, val, ret)

		ret = Optional[[]string](nil)
		assert.Equal(t, []string(nil), ret)
	})
}

func TestGetValue(t *testing.T) {
	t.Parallel()

	t.Run("int", func(t *testing.T) {
		t.Parallel()

		key := "key"
		val := 3
		valueMap := map[string]any{key: val}

		ret, ok := GetValue[int](valueMap, key)
		assert.True(t, ok)
		assert.Equal(t, val, ret)

		_, ok = GetValue[int](valueMap, "foo")
		assert.False(t, ok)

		_, ok = GetValue[string](valueMap, key)
		assert.False(t, ok)
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		key := "key"
		val := "val"
		valueMap := map[string]any{key: val}

		ret, ok := GetValue[string](valueMap, key)
		assert.True(t, ok)
		assert.Equal(t, val, ret)

		_, ok = GetValue[string](valueMap, "foo")
		assert.False(t, ok)

		_, ok = GetValue[int](valueMap, key)
		assert.False(t, ok)
	})

	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		key := "key"
		val := []string{"foo", "bar"}
		valueMap := map[string]any{key: val}

		ret, ok := GetValue[[]string](valueMap, key)
		assert.True(t, ok)
		assert.Equal(t, val, ret)

		_, ok = GetValue[[]string](valueMap, "foo")
		assert.False(t, ok)

		_, ok = GetValue[int](valueMap, key)
		assert.False(t, ok)
	})
}
