package reconciler

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HasStaleCondition(cond *metav1.Condition, timeout time.Duration) bool {
	if cond == nil {
		return false
	}
	return time.Now().After(cond.LastTransitionTime.Add(timeout))
}

func ConditionTrue(cond *metav1.Condition) bool {
	return cond != nil && cond.Status == metav1.ConditionTrue
}
