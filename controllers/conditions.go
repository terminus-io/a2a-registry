package controllers

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// A2AFinalizer is the finalizer name used by both A2AAgent and A2ARegistry controllers.
	A2AFinalizer = "a2a.io/finalizer"

	// ConditionTypeApproved is the condition type for agent approval status.
	ConditionTypeApproved = "Approved"
)

// MergeCondition updates an existing condition with the same type or appends a new one.
func MergeCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}
