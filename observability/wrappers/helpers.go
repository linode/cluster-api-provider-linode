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

func GetValue[T any](m map[string]any, key string) (T, bool) {
	var zero T

	if val, ok := m[key]; ok {
		if val, ok := val.(T); ok {
			return val, true
		}
	}

	return zero, false
}

func Optional[T any](val *T) T {
	var zero T

	if val != nil {
		return *val
	}

	return zero
}
