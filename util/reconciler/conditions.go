package reconciler

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
)

func ConditionTrue(from conditions.Getter, typ string) bool {
	return HasConditionStatus(from, typ, metav1.ConditionTrue)
}

func HasConditionStatus(from conditions.Getter, typ string, status metav1.ConditionStatus) bool {
	cond := conditions.Get(from, typ)
	if cond == nil {
		return false
	}

	return cond.Status == status
}

func RecordDecayingCondition(to conditions.Setter, typ string, reason, message string, timeout time.Duration) bool {
	if HasStaleCondition(to, typ, timeout) {
		return true
	}

	return false
}

func HasStaleCondition(from conditions.Getter, typ string, timeout time.Duration) bool {
	cond := conditions.Get(from, typ)
	if cond == nil {
		return false
	}

	return time.Now().After(cond.LastTransitionTime.Add(timeout))
}
