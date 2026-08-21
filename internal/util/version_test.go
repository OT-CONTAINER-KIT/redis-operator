package util

import "testing"

func TestRedisMajorVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    int
	}{
		{name: "v6", version: "v6", want: 6},
		{name: "v7", version: "v7", want: 7},
		{name: "v8", version: "v8", want: 8},
		{name: "v10 is not lexically compared", version: "v10", want: 10},
		{name: "major without leading v", version: "7", want: 7},
		{name: "uppercase V", version: "V8", want: 8},
		{name: "minor version is ignored", version: "v7.2", want: 7},
		{name: "patch version is ignored", version: "v8.0.1", want: 8},
		{name: "minor version without leading v", version: "1.0", want: 1},
		{name: "empty falls back to the default", version: "", want: 7},
		{name: "garbage falls back to the default", version: "latest", want: 7},
		{name: "lone v falls back to the default", version: "v", want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedisMajorVersion(tt.version); got != tt.want {
				t.Errorf("RedisMajorVersion(%q) = %d, want %d", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsRedisVersionAtLeastV7(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "v5 is below 7", version: "v5", want: false},
		{name: "v6 is below 7", version: "v6", want: false},
		{name: "v7 is supported", version: "v7", want: true},
		{name: "v8 is supported", version: "v8", want: true},
		{name: "v10 is supported", version: "v10", want: true},
		{name: "bare 7 is supported", version: "7", want: true},
		{name: "v7.2 is supported", version: "v7.2", want: true},
		{name: "1.0 is below 7", version: "1.0", want: false},
		{name: "empty defaults to v7 behaviour", version: "", want: true},
		{name: "garbage defaults to v7 behaviour", version: "not-a-version", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRedisVersionAtLeastV7(tt.version); got != tt.want {
				t.Errorf("IsRedisVersionAtLeastV7(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
