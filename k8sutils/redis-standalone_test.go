package k8sutils

import (
	"os"
	"path/filepath"
	"testing"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/pointer"
)

func Test_generateRedisStandaloneParams(t *testing.T) {
	path := filepath.Join("..", "tests", "testdata", "redis-standalone.yaml")
	expected := statefulSetParameters{
		Replicas:       pointer.Int32(1),
		ClusterMode:    false,
		NodeConfVolume: false,
		// Metadata: metav1.ObjectMeta{
		// 	Name:      "redis-standalone",
		// 	Namespace: "redis",
		// 	Labels: map[string]string{
		// 		"app": "redis-standalone"},
		// 	Annotations: map[string]string{
		// 		"opstreelabs.in.redis": "true"},
		// },
		NodeSelector: map[string]string{
			"node-role.kubernetes.io/infra": "worker"},
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: pointer.Int64(1000),
			FSGroup:   pointer.Int64(1000),
		},
		PriorityClassName: "high-priority",
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"worker"},
								},
							},
						},
					},
				},
			},
		},
		Tolerations: &[]corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
		},
		PersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: pointer.String("standard"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
		EnableMetrics:                 true,
		ImagePullSecrets:              &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ExternalConfig:                pointer.String("redis-external-config"),
		ServiceAccountName:            pointer.String("redis-sa"),
		TerminationGracePeriodSeconds: pointer.Int64(30),
		IgnoreAnnotations:             []string{"opstreelabs.in/ignore"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &redisv1beta2.Redis{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisStandaloneParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}
