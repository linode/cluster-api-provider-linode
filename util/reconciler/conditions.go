package reconciler

import (
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

func OneOfConditionsTrue(from conditions.Getter, typs ...clusterv1.ConditionType) bool {
	for _, typ := range typs {
		if conditions.IsTrue(from, typ) {
			return true
		}
	}

	return false
}

func RecordDecayingCondition(to conditions.Setter, typ clusterv1.ConditionType, reason, message string, timeout time.Duration) bool {
	conditions.MarkFalse(to, typ, reason, clusterv1.ConditionSeverityWarning, message)

	if HasStaleCondition(to, typ, timeout) {
		conditions.MarkFalse(to, typ, reason, clusterv1.ConditionSeverityError, message)
		return true
	}

	return false
}

func HasStaleCondition(from conditions.Getter, typ clusterv1.ConditionType, timeout time.Duration) bool {
	cond := conditions.Get(from, typ)
	if cond == nil {
		return false
	}

	return time.Now().After(cond.LastTransitionTime.Add(timeout))
}
