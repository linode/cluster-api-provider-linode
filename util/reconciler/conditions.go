package reconciler

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

func ConditionTrue(from conditions.Getter, typ clusterv1.ConditionType) bool {
	return HasConditionStatus(from, typ, "True")
}

func HasConditionStatus(from conditions.Getter, typ clusterv1.ConditionType, status corev1.ConditionStatus) bool {
	cond := conditions.Get(from, typ)
	if cond == nil {
		return false
	}

	return cond.Status == status
}

func HasConditionSeverity(from conditions.Getter, typ clusterv1.ConditionType, severity clusterv1.ConditionSeverity) bool {
	cond := conditions.Get(from, typ)
	if cond == nil {
		return false
	}

	return cond.Severity == severity
}

func RecordDecayingCondition(to conditions.Setter, typ clusterv1.ConditionType, reason, message string, timeout time.Duration) bool {
	conditions.MarkFalse(to, typ, reason, clusterv1.ConditionSeverityWarning, "%s", message)

	if HasStaleCondition(to, typ, timeout) {
		conditions.MarkFalse(to, typ, reason, clusterv1.ConditionSeverityError, "%s", message)
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
