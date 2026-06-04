package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergeCondition_AppendsNewCondition(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}

	newCondition := metav1.Condition{
		Type:   "HealthChecked",
		Status: metav1.ConditionTrue,
	}

	result := MergeCondition(conditions, newCondition)
	if len(result) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(result))
	}
	if result[1].Type != "HealthChecked" {
		t.Errorf("expected HealthChecked condition, got %s", result[1].Type)
	}
}

func TestMergeCondition_ReplacesExistingCondition(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Message: "old"},
	}

	newCondition := metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Message: "new",
	}

	result := MergeCondition(conditions, newCondition)
	if len(result) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(result))
	}
	if result[0].Message != "new" {
		t.Errorf("expected message 'new', got '%s'", result[0].Message)
	}
	if result[0].Status != metav1.ConditionFalse {
		t.Errorf("expected status False, got %s", result[0].Status)
	}
}

func TestMergeCondition_EmptySlice(t *testing.T) {
	newCondition := metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
	}

	result := MergeCondition(nil, newCondition)
	if len(result) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(result))
	}
	if result[0].Type != "Ready" {
		t.Errorf("expected Ready condition, got %s", result[0].Type)
	}
}

func TestMergeCondition_PreservesOrderOnAppend(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "First", Status: metav1.ConditionTrue},
		{Type: "Second", Status: metav1.ConditionTrue},
	}

	newCondition := metav1.Condition{
		Type:   "Third",
		Status: metav1.ConditionTrue,
	}

	result := MergeCondition(conditions, newCondition)
	if len(result) != 3 {
		t.Fatalf("expected 3 conditions, got %d", len(result))
	}
	if result[0].Type != "First" || result[1].Type != "Second" || result[2].Type != "Third" {
		t.Errorf("order not preserved: %v", result)
	}
}

func TestA2AFinalizerConstant(t *testing.T) {
	if A2AFinalizer != "a2a.io/finalizer" {
		t.Errorf("expected A2AFinalizer to be 'a2a.io/finalizer', got '%s'", A2AFinalizer)
	}
}

func TestConditionTypeApprovedConstant(t *testing.T) {
	if ConditionTypeApproved != "Approved" {
		t.Errorf("expected ConditionTypeApproved to be 'Approved', got '%s'", ConditionTypeApproved)
	}
}
