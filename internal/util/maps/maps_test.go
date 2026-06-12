package maps_test

import (
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
)

func TestIsSubset(t *testing.T) {
	tests := []struct {
		name    string
		toCheck map[string]string
		fullSet map[string]string
		want    bool
	}{
		{
			name:    "subset",
			toCheck: map[string]string{"a": "1", "b": "2"},
			fullSet: map[string]string{"a": "1", "b": "2", "c": "3"},
			want:    true,
		},
		{
			name:    "exact match",
			toCheck: map[string]string{"x": "y"},
			fullSet: map[string]string{"x": "y"},
			want:    true,
		},
		{
			name:    "empty subset",
			toCheck: map[string]string{},
			fullSet: map[string]string{"a": "1"},
			want:    true,
		},
		{
			name:    "missing key",
			toCheck: map[string]string{"missing": "value"},
			fullSet: map[string]string{"present": "value"},
			want:    false,
		},
		{
			name:    "mismatched value",
			toCheck: map[string]string{"k": "different"},
			fullSet: map[string]string{"k": "expected"},
			want:    false,
		},
		{
			name:    "subset larger than full set",
			toCheck: map[string]string{"a": "1", "b": "2"},
			fullSet: map[string]string{"a": "1"},
			want:    false,
		},
		{
			name:    "nil maps",
			toCheck: nil,
			fullSet: nil,
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maps.IsSubset(tt.toCheck, tt.fullSet)
			if got != tt.want {
				t.Errorf("IsSubset() = %v, want %v", got, tt.want)
			}
		})
	}
}
