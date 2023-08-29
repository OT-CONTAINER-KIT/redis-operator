package v1beta1

import (
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this Redis to the Hub version (vbeta1).
func (src *Redis) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*redisv1beta2.Redis)

	return nil
}

// ConvertFrom converts from the Hub version (vbeta1) to this version.
func (dst *Redis) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*redisv1beta2.Redis)

	return nil
}
