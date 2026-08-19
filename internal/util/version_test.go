package util

import "testing"

func TestIsRedisVersion7OrNewer(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "v6 takes the legacy path", version: "v6", want: false},
		{name: "v7 takes the modern path", version: "v7", want: true},
		{name: "v8 takes the modern path", version: "v8", want: true},
		{name: "v9 takes the modern path", version: "v9", want: true},
		{name: "future double-digit major", version: "v10", want: true},
		{name: "bare numeric major below 7", version: "6", want: false},
		{name: "bare numeric major at boundary", version: "7", want: true},
		{name: "bare numeric major above 7", version: "8", want: true},
		{name: "version with minor and patch", version: "v7.2.1", want: true},
		{name: "older version with minor and patch", version: "v6.2.20", want: false},
		{name: "uppercase prefix", version: "V8", want: true},
		{name: "surrounding whitespace", version: " v8 ", want: true},
		{name: "empty defaults to v7 (modern)", version: "", want: true},
		{name: "garbage defaults to v7 (modern)", version: "garbage", want: true},
		{name: "lone prefix defaults to v7 (modern)", version: "v", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRedisVersion7OrNewer(tt.version); got != tt.want {
				t.Errorf("IsRedisVersion7OrNewer(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
