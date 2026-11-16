package v1alpha2

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCalculateRRsetSyncStatusAndGeneration(t *testing.T) {
	invalidGen1 := metav1.Condition{Type: "Valid", Status: metav1.ConditionFalse, ObservedGeneration: 1}
	invalidGen2 := metav1.Condition{Type: "Valid", Status: metav1.ConditionFalse, ObservedGeneration: 2}
	invalidGen3 := metav1.Condition{Type: "Valid", Status: metav1.ConditionFalse, ObservedGeneration: 3}
	validGen1 := metav1.Condition{Type: "Valid", Status: metav1.ConditionTrue, ObservedGeneration: 1}
	validGen2 := metav1.Condition{Type: "Valid", Status: metav1.ConditionTrue, ObservedGeneration: 2}
	validGen3 := metav1.Condition{Type: "Valid", Status: metav1.ConditionTrue, ObservedGeneration: 3}
	processedGen1 := metav1.Condition{Type: "Processed", Status: metav1.ConditionTrue, ObservedGeneration: 1}
	processedGen2 := metav1.Condition{Type: "Processed", Status: metav1.ConditionTrue, ObservedGeneration: 2}
	unprocessedGen1 := metav1.Condition{Type: "Processed", Status: metav1.ConditionFalse, ObservedGeneration: 1}
	unprocessedGen2 := metav1.Condition{Type: "Processed", Status: metav1.ConditionFalse, ObservedGeneration: 2}
	availableGen1 := metav1.Condition{Type: "Available", Status: metav1.ConditionTrue, ObservedGeneration: 1}

	tests := []struct {
		name               string
		status             *RRsetStatus
		generation         int64
		expected           string
		expectedGeneration int64
	}{
		{name: "Never synced and invalid", status: &RRsetStatus{Conditions: []metav1.Condition{invalidGen1}}, generation: 1, expected: "Invalid", expectedGeneration: 1},
		{name: "Never synced and valid", status: &RRsetStatus{Conditions: []metav1.Condition{validGen1}}, generation: 1, expected: "Valid", expectedGeneration: 1},
		{name: "Valid but Unprocessed in current generation", status: &RRsetStatus{Conditions: []metav1.Condition{validGen1, unprocessedGen1}}, generation: 1, expected: "Unprocessed", expectedGeneration: 1},
		{name: "Valid and Processed in current generation", status: &RRsetStatus{Conditions: []metav1.Condition{validGen1, processedGen1}}, generation: 1, expected: "Processed", expectedGeneration: 1},
		{name: "Synced in current generation", status: &RRsetStatus{Conditions: []metav1.Condition{validGen1, processedGen1, availableGen1}}, generation: 1, expected: "Synced", expectedGeneration: 1},
		{name: "Synced in previous generation, invalid in current", status: &RRsetStatus{Conditions: []metav1.Condition{invalidGen2, processedGen1, availableGen1}}, generation: 2, expected: "Stale", expectedGeneration: 1},
		{name: "Synced in previous generation, valid in current", status: &RRsetStatus{Conditions: []metav1.Condition{validGen2, processedGen1, availableGen1}}, generation: 2, expected: "Stale", expectedGeneration: 1},
		{name: "Synced in previous generation, valid and processed in current", status: &RRsetStatus{Conditions: []metav1.Condition{validGen2, processedGen2, availableGen1}}, generation: 2, expected: "Stale", expectedGeneration: 1},
		{name: "Unprocessed in previous generation and valid", status: &RRsetStatus{Conditions: []metav1.Condition{validGen2, unprocessedGen1}}, generation: 2, expected: "Unprocessed", expectedGeneration: 1},
		{name: "Unprocessed in previous generation and invalid", status: &RRsetStatus{Conditions: []metav1.Condition{invalidGen2, unprocessedGen1}}, generation: 2, expected: "Invalid", expectedGeneration: 2},
		{name: "Synced in first generation, unprocessed and invalid in future generations", status: &RRsetStatus{Conditions: []metav1.Condition{invalidGen3, unprocessedGen2, availableGen1}}, generation: 3, expected: "Stale", expectedGeneration: 1},
		{name: "Synced in first generation, unprocessed in second, valid in current", status: &RRsetStatus{Conditions: []metav1.Condition{validGen3, unprocessedGen2, availableGen1}}, generation: 3, expected: "Stale", expectedGeneration: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, generation := calculateRRsetSyncStatusAndGeneration(test.status, test.generation)
			if result != test.expected {
				t.Errorf("expected %s, got %s", test.expected, result)
			}
			if generation != test.expectedGeneration {
				t.Errorf("expected generation %d, got %d", test.expectedGeneration, generation)
			}
		})
	}
}
