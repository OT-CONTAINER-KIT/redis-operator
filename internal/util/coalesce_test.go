package util

import "testing"

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name        string
		val         int
		defaultVal  int
		expectedVal int
	}{
		{
			name:        "Value is zero, default value is provided",
			val:         0,
			defaultVal:  10,
			expectedVal: 10,
		},
		{
			name:        "Value is non-zero, default value is provided",
			val:         5,
			defaultVal:  10,
			expectedVal: 5,
		},
		{
			name:        "Value is zero, default value is zero",
			val:         0,
			defaultVal:  0,
			expectedVal: 0,
		},
		{
			name:        "Value is non-zero, default value is zero",
			val:         5,
			defaultVal:  0,
			expectedVal: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualVal := Coalesce(tt.val, tt.defaultVal)
			if actualVal != tt.expectedVal {
				t.Errorf("Expected value %v, but got %v", tt.expectedVal, actualVal)
			}
		})
	}
}
