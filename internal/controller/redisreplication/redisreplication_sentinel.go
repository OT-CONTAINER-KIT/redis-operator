package redisreplication

import (
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func newSentinelService(rr *rrvb2.RedisReplication) corev1.Service {
	labels := common.GetRedisLabels(
		rr.SentinelStatefulSet(),
		common.SetupTypeSentinel,
		"sentinel",
		rr.GetLabels(),
	)

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rr.SentinelHLService(),
			Namespace: rr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "sentinel",
					Port:       26379,
					TargetPort: intstr.FromInt(26379),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
