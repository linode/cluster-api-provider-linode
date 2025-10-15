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

package v1alpha2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		length int
		delta  int
		del    string
	}{
		{name: "empty", length: 0, del: "type0"},
		{name: "delete-only", length: 1, del: "type0", delta: -1},
		{name: "delete-first", length: 3, del: "type0", delta: -1},
		{name: "delete-last", length: 3, del: "type2", delta: -1},
		{name: "delete-missing", length: 3, del: "type3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			conds := make([]metav1.Condition, tc.length)
			for i := 0; i < tc.length; i++ {
				conds[i] = metav1.Condition{Type: fmt.Sprintf("type%d", i)}
			}
			lm := &LinodeMachine{
				Status: LinodeMachineStatus{
					Conditions: conds,
				},
			}

			lm.DeleteCondition(tc.del)
			assert.NotContains(t, lm.Status.Conditions, tc.del)
			assert.Len(t, lm.Status.Conditions, tc.length+tc.delta)
		})
	}
}
