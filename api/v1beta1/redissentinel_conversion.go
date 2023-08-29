package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this RedisSentinel to the Hub version (vbeta1).
func (src *RedisSentinel) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.RedisSentinel)

	return nil
}

// ConvertFrom converts from the Hub version (vbeta1) to this version.
func (dst *RedisSentinel) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.RedisSentinel)

	return nil
}
