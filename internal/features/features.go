package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// GenerateConfigInInitContainer enables generating Redis configuration using an init container
	// instead of a regular container
	GenerateConfigInInitContainer featuregate.Feature = "GenerateConfigInInitContainer"
)

// DefaultRedisOperatorFeatureGates consists of all known Redis operator feature gates.
// To add a new feature, define a key for it above and add it here.
var DefaultRedisOperatorFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	GenerateConfigInInitContainer: {Default: false, PreRelease: featuregate.Alpha},
}

// MutableFeatureGate is a feature gate that can be dynamically set
var MutableFeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

//nolint:gochecknoinits
func init() {
	runtime.Must(MutableFeatureGate.Add(DefaultRedisOperatorFeatureGates))
}

// Enabled checks if a feature is enabled
func Enabled(feature featuregate.Feature) bool {
	return MutableFeatureGate.Enabled(feature)
}
