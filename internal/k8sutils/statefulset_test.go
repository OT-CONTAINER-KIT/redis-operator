package k8sutils

import (
	"context"
	"path"
	"strconv"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/consts"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestGenerateAuthAndTLSArgs(t *testing.T) {
	tests := []struct {
		name         string
		enableAuth   bool
		enableTLS    bool
		expectedAuth string
		expectedTLS  string
	}{
		{"NoAuthNoTLS", false, false, "", ""},
		{"AuthOnly", true, false, " -a \"${REDIS_PASSWORD}\"", ""},
		{"TLSOnly", false, true, "", " --tls --cert \"${REDIS_TLS_CERT}\" --key \"${REDIS_TLS_CERT_KEY}\" --cacert \"${REDIS_TLS_CA_KEY}\""},
		{"AuthAndTLS", true, true, " -a \"${REDIS_PASSWORD}\"", " --tls --cert \"${REDIS_TLS_CERT}\" --key \"${REDIS_TLS_CERT_KEY}\" --cacert \"${REDIS_TLS_CA_KEY}\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authArgs, tlsArgs := GenerateAuthAndTLSArgs(tt.enableAuth, tt.enableTLS)
			if authArgs != tt.expectedAuth {
				t.Errorf("expected auth args %q, got %q", tt.expectedAuth, authArgs)
			}
			if tlsArgs != tt.expectedTLS {
				t.Errorf("expected TLS args %q, got %q", tt.expectedTLS, tlsArgs)
			}
		})
	}
}

func TestGeneratePreStopCommand(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		expectEmpty bool
	}{
		{"ClusterRole", "cluster", false},
		{"ReplicationRole", "replication", true},
		{"SentinelRole", "sentinel", true},
		{"StandaloneRole", "standalone", true},
		{"UnknownRole", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratePreStopCommand(tt.role, true, true)
			if (result == "") != tt.expectEmpty {
				t.Errorf("expected empty: %v, got: %q", tt.expectEmpty, result)
			}
		})
	}
}

func TestGenerateContainerDefAddsMaxMemoryEnv(t *testing.T) {
	percent := 80
	memLimit := resource.MustParse("512Mi")
	containers := generateContainerDef(
		"redis",
		containerParameters{
			Role:  "redis",
			Image: "redis:latest",
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: memLimit,
				},
			},
			MaxMemoryPercentOfLimit: &percent,
		},
		false,
		false,
		false,
		nil,
		nil,
		nil,
		nil,
	)

	require.Len(t, containers, 1)
	expectedValue := strconv.FormatInt(memLimit.Value()*int64(percent)/100, 10)
	assert.Contains(t, containers[0].Env, corev1.EnvVar{Name: consts.ENV_KEY_REDIS_MAX_MEMORY, Value: expectedValue})
}

func TestGetVolumeMount(t *testing.T) {
	tests := []struct {
		name               string
		persistenceEnabled *bool
		clusterMode        bool
		nodeConfVolume     bool
		externalConfig     *string
		mountpath          []corev1.VolumeMount
		tlsConfig          *common.TLSConfig
		aclConfig          *common.ACLConfig
		expectedMounts     []corev1.VolumeMount
	}{
		{
			name:               "1. All false or nil",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{},
		},
		{
			name:               "2. Persistence enabled with cluster mode and node conf",
			persistenceEnabled: ptr.To(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts: []corev1.VolumeMount{
				{
					Name:      "persistent-volume",
					MountPath: "/data",
				},
				{
					Name:      "node-conf",
					MountPath: "/node-conf",
				},
			},
		},
		{
			name:               "3. Persistence enabled with cluster mode and external config",
			persistenceEnabled: ptr.To(true),
			clusterMode:        true,
			nodeConfVolume:     false,
			externalConfig:     ptr.To("some-config"),
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts: []corev1.VolumeMount{
				{
					Name:      "persistent-volume",
					MountPath: "/data",
				},
				{
					Name:      "external-config",
					MountPath: "/etc/redis/external.conf.d",
				},
			},
		},
		{
			name:               "4. Persistence enabled, cluster mode false, node conf true, no tls/acl, with mountpath",
			persistenceEnabled: ptr.To(true),
			clusterMode:        false,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig:      nil,
			aclConfig:      nil,
			expectedMounts: []corev1.VolumeMount{{Name: "persistent-volume", MountPath: "/data"}, {Name: "additional-mount", MountPath: "/additional"}},
		},
		{
			name:               "5. Only tls enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          &common.TLSConfig{},
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{{Name: "tls-certs", MountPath: "/tls", ReadOnly: true}},
		},
		{
			name:               "6. Only acl enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          &common.ACLConfig{},
			expectedMounts:     []corev1.VolumeMount{{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"}},
		},
		{
			name:               "7. Everything enabled except externalConfig",
			persistenceEnabled: ptr.To(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig: &common.TLSConfig{},
			aclConfig: &common.ACLConfig{},
			expectedMounts: []corev1.VolumeMount{
				{Name: "persistent-volume", MountPath: "/data"},
				{Name: "node-conf", MountPath: "/node-conf"},
				{Name: "tls-certs", MountPath: "/tls", ReadOnly: true},
				{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"},
				{Name: "additional-mount", MountPath: "/additional"},
			},
		},
		{
			name:               "8. Only externalConfig enabled",
			persistenceEnabled: nil,
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     ptr.To("some-config"),
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          nil,
			expectedMounts:     []corev1.VolumeMount{{Name: "external-config", MountPath: "/etc/redis/external.conf.d"}},
		},
		{
			name:               "9. Persistence enabled, cluster mode true, node conf true, only acl enabled",
			persistenceEnabled: ptr.To(true),
			clusterMode:        true,
			nodeConfVolume:     true,
			externalConfig:     nil,
			mountpath:          []corev1.VolumeMount{},
			tlsConfig:          nil,
			aclConfig:          &common.ACLConfig{},
			expectedMounts: []corev1.VolumeMount{
				{Name: "persistent-volume", MountPath: "/data"},
				{Name: "node-conf", MountPath: "/node-conf"},
				{Name: "acl-secret", MountPath: "/etc/redis/user.acl", SubPath: "user.acl"},
			},
		},
		{
			name:               "10. Persistence enabled, cluster mode false, node conf false, only tls enabled with mountpath",
			persistenceEnabled: ptr.To(true),
			clusterMode:        false,
			nodeConfVolume:     false,
			externalConfig:     nil,
			mountpath: []corev1.VolumeMount{
				{
					Name:      "additional-mount",
					MountPath: "/additional",
				},
			},
			tlsConfig:      &common.TLSConfig{},
			aclConfig:      nil,
			expectedMounts: []corev1.VolumeMount{{Name: "persistent-volume", MountPath: "/data"}, {Name: "tls-certs", MountPath: "/tls", ReadOnly: true}, {Name: "additional-mount", MountPath: "/additional"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVolumeMount("persistent-volume", tt.persistenceEnabled, tt.clusterMode, tt.nodeConfVolume, tt.externalConfig, tt.mountpath, tt.tlsConfig, tt.aclConfig)
			assert.ElementsMatch(t, tt.expectedMounts, got)
		})
	}
}

func Test_GetStatefulSet(t *testing.T) {
	tests := []struct {
		name         string
		sts          appsv1.StatefulSet
		stsName      string
		stsNamespace string
		present      bool
	}{
		{
			name: "StatefulSet present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},
			stsName:      "test-sts",
			stsNamespace: "test-ns",
			present:      true,
		},
		{
			name:         "StatefulSet not present",
			sts:          appsv1.StatefulSet{},
			stsName:      "test-sts",
			stsNamespace: "test-ns",
			present:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := k8sClientFake.NewSimpleClientset(test.sts.DeepCopy())
			_, err := GetStatefulSet(context.TODO(), client, test.stsNamespace, test.stsName)
			if test.present {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func Test_createStatefulSet(t *testing.T) {
	tests := []struct {
		name    string
		sts     appsv1.StatefulSet
		present bool
	}{
		{
			name: "StatefulSet present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},

			present: true,
		},
		{
			name: "StatefulSet not present",
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
			},
			present: false,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			var client *k8sClientFake.Clientset
			if test.present {
				client = k8sClientFake.NewSimpleClientset(test.sts.DeepCopy())
			} else {
				client = k8sClientFake.NewSimpleClientset()
			}
			err := createStatefulSet(context.TODO(), client, test.sts.GetNamespace(), &test.sts)
			if test.present {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestUpdateStatefulSet(t *testing.T) {
	tests := []struct {
		name              string
		existingStsSpec   appsv1.StatefulSetSpec
		updatedStsSpec    appsv1.StatefulSetSpec
		recreateSts       bool
		deletePropagation metav1.DeletionPropagation
		stsPresent        bool
		expectErr         error
	}{
		{
			name: "Update StatefulSet without recreate in existing Statefulset",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(3)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(5)),
			},
			recreateSts: false,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet with recreate in existing Statefulset",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: true,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet without recreate StatefulSet is not present",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: false,
			stsPresent:  false,
			expectErr:   kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test-sts"),
		},
		{
			name: "Update StatefulSet without recreate StatefulSet",
			existingStsSpec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"name": "redis",
						},
					},
				},
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"name": "redis-standalone",
						},
					},
				},
			},
			recreateSts: false,
			stsPresent:  true,
		},
		{
			name: "Update StatefulSet failed with Invalid Reason",
			existingStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			updatedStsSpec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			recreateSts: true,
			stsPresent:  false,
			expectErr:   kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test-sts"),
		},
	}

	assert := assert.New(t)

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			existingSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
				Spec: *test.existingStsSpec.DeepCopy(),
			}
			updatedSts := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-ns",
				},
				Spec: *test.updatedStsSpec.DeepCopy(),
			}
			var client *k8sClientFake.Clientset
			if test.stsPresent {
				client = k8sClientFake.NewSimpleClientset(existingSts.DeepCopyObject())
			} else {
				client = k8sClientFake.NewSimpleClientset()
			}
			err := updateStatefulSet(context.TODO(), client, updatedSts.GetNamespace(), &updatedSts, test.recreateSts, &test.deletePropagation)
			if test.expectErr != nil {
				assert.Error(err, "Expected Error while updating Statefulset")
				assert.Equal(test.expectErr, err)
			} else {
				assert.NoError(err, "Error while updating Statefulset")
			}
			if err == nil {
				getUpdatedSts, err := client.AppsV1().StatefulSets(updatedSts.GetNamespace()).Get(context.TODO(), updatedSts.GetName(), metav1.GetOptions{})
				assert.NoError(err, "Error getting Updted StatefulSet")
				assert.NotEqual(getUpdatedSts.DeepCopy(), existingSts.DeepCopy(), "StatefulSet not updated")
			}
		})
	}
}

func TestCreateOrUpdateStateFul(t *testing.T) {
	tests := []struct {
		name                string
		stsParams           statefulSetParameters
		stsOwnerDef         metav1.OwnerReference
		initContainerParams initContainerParameters
		containerParams     containerParameters
		sidecar             *[]common.Sidecar
		existingStatefulSet appsv1.StatefulSetSpec
		updatedStatefulSet  appsv1.StatefulSetSpec
		stsPresent          bool
		expectErr           error
	}{
		{
			name: "Test1_Create_Statefulset",
			stsParams: statefulSetParameters{
				Replicas: ptr.To(int32(4)),
			},
			stsOwnerDef: metav1.OwnerReference{
				Name: "test-sts",
			},
			initContainerParams: initContainerParameters{
				Image: "redis-init:latest",
			},
			containerParams: containerParameters{
				Image: "redis:latest",
				ReadinessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
				LivenessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
			},
			sidecar: &[]common.Sidecar{
				{
					Name:    "redis-sidecare",
					Command: []string{"/bin/bash", "-c", "/app/restore.bash"},
				},
			},
			existingStatefulSet: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			stsPresent: false,
		},
		{
			name: "Test2_udpate_Statefulset",
			stsParams: statefulSetParameters{
				Replicas: ptr.To(int32(4)),
			},
			stsOwnerDef: metav1.OwnerReference{
				Name: "test-sts",
			},
			initContainerParams: initContainerParameters{
				Image: "redis-init:latest",
			},
			containerParams: containerParameters{
				Image: "redis:latest",
				ReadinessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
				LivenessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
			},
			sidecar: &[]common.Sidecar{
				{
					Name:    "redis-sidecare",
					Command: []string{"/bin/bash", "-c", "/app/restore.bash"},
				},
			},
			existingStatefulSet: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			updatedStatefulSet: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(6)),
			},
			stsPresent: true,
		},
		{
			name: "Test3_Create_Statefulset_With_Error",
			stsParams: statefulSetParameters{
				Replicas: ptr.To(int32(4)),
			},
			stsOwnerDef: metav1.OwnerReference{
				Name: "test-sts",
			},
			initContainerParams: initContainerParameters{
				Image: "redis-init:latest",
			},
			containerParams: containerParameters{
				Image: "redis:latest",
				ReadinessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
				LivenessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(5),
				},
			},
			sidecar: &[]common.Sidecar{},
			existingStatefulSet: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(4)),
			},
			updatedStatefulSet: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(-6)),
			},
			stsPresent: false,
			expectErr:  kerrors.NewBadRequest("Invalid Value of Replicas"),
		},
	}

	assert := assert.New(t)

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			var client *k8sClientFake.Clientset

			if test.stsPresent {
				existingSts := appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sts",
						Namespace: "test-ns",
					},
					Spec: *test.existingStatefulSet.DeepCopy(),
				}

				updatedSts := appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sts",
						Namespace: "test-ns",
					},
					Spec: *test.updatedStatefulSet.DeepCopy(),
				}
				if test.stsPresent {
					client = k8sClientFake.NewSimpleClientset(existingSts.DeepCopy())
				} else {
					client = k8sClientFake.NewSimpleClientset()
				}
				err := CreateOrUpdateStateFul(context.TODO(), client, updatedSts.GetNamespace(), updatedSts.ObjectMeta, test.stsParams, test.stsOwnerDef, test.initContainerParams, test.containerParams, test.sidecar)
				if test.expectErr != nil {
					assert.Error(err, "Expected Error while updating Statefulset")
					assert.Equal(test.expectErr, err)
				} else {
					assert.NoError(err, "Error while updating Statefulset")
				}
				if err == nil {
					getUpdatedSts, err := client.AppsV1().StatefulSets(updatedSts.GetNamespace()).Get(context.TODO(), updatedSts.GetName(), metav1.GetOptions{})
					assert.NoError(err)
					assert.NotEqual(getUpdatedSts.DeepCopy(), existingSts.DeepCopy(), "StatefulSet Updated")
				}
			} else {
				updatedSts := appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sts",
						Namespace: "",
					},
					Spec: *test.updatedStatefulSet.DeepCopy(),
				}

				client = k8sClientFake.NewSimpleClientset()

				err := CreateOrUpdateStateFul(context.TODO(), client, updatedSts.GetNamespace(), updatedSts.ObjectMeta, test.stsParams, test.stsOwnerDef, test.initContainerParams, test.containerParams, test.sidecar)
				assert.Nil(err)
			}
		})
	}
}

func TestCreateOrUpdateResizingPVC(t *testing.T) {
	tests := []struct {
		name                     string
		startingPVCSize          string
		newPVCSize               string
		recreateStatefulSet      bool
		expectedVCTUpdate        bool
		expectedAnnotationUpdate bool
		expectErr                error
	}{
		{
			name:                     "NoPVCSizeChangeResultsInNoSTSUpdate",
			startingPVCSize:          "2Gi",
			newPVCSize:               "2Gi",
			recreateStatefulSet:      true,
			expectedVCTUpdate:        false,
			expectedAnnotationUpdate: false,
			expectErr:                nil,
		},
		{
			name:                     "NoPVCSizeChangeResultsInNoSTSUpdate_2",
			startingPVCSize:          "2Gi",
			newPVCSize:               "2Gi",
			recreateStatefulSet:      false,
			expectedVCTUpdate:        false,
			expectedAnnotationUpdate: false,
			expectErr:                nil,
		},
		{
			name:                     "PVCSizeChangeResultsInSTSUpdate",
			startingPVCSize:          "2Gi",
			newPVCSize:               "3Gi",
			recreateStatefulSet:      true,
			expectedVCTUpdate:        true,
			expectedAnnotationUpdate: true,
			expectErr:                nil,
		},
		{
			name:                     "PVCResizeChangesAnnotationOnlyIfRecreateNotEnabled",
			startingPVCSize:          "2Gi",
			newPVCSize:               "3Gi",
			recreateStatefulSet:      false,
			expectedVCTUpdate:        false,
			expectedAnnotationUpdate: true,
			expectErr:                nil,
		},
	}

	stsOwnerDef := metav1.OwnerReference{
		Name:       "test-sts",
		Kind:       "StatefulSet",
		APIVersion: "apps/v1",
		UID:        "12345",
	}
	initContainerParams := initContainerParameters{Image: "redis-init:latest"}
	containerParams := containerParameters{
		PersistenceEnabled: ptr.To(true),
	}
	sidecar := &[]common.Sidecar{}
	objMeta := metav1.ObjectMeta{
		Name:      "test-sts",
		Namespace: "test-ns",
		UID:       "12345",
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			stsParams := statefulSetParameters{
				Replicas:            ptr.To(int32(4)),
				RecreateStatefulSet: test.recreateStatefulSet,
				NodeConfVolume:      true,
				NodeConfPersistentVolumeClaim: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-conf",
						Annotations: map[string]string{
							"redis.opstreelabs.in":       "true",
							"redis.opstreelabs.instance": "test-sts",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
				PersistentVolumeClaim: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: objMeta.Name,
						Annotations: map[string]string{
							"redis.opstreelabs.in":       "true",
							"redis.opstreelabs.instance": "test-sts",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(test.startingPVCSize),
							},
						},
					},
				},
			}
			client := k8sClientFake.NewSimpleClientset()
			// create the STS with the initial params and PVC so that we know there shouldn't be any other differences
			err := CreateOrUpdateStateFul(context.Background(), client, objMeta.Namespace, objMeta,
				stsParams, stsOwnerDef, initContainerParams, containerParams, sidecar)
			require.NoError(t, err, "Error while creating Statefulset")
			getCreatedSts, err := client.AppsV1().StatefulSets(objMeta.Namespace).Get(context.Background(), objMeta.Name, metav1.GetOptions{})
			require.NoError(t, err, "Error getting StatefulSet")

			// only change the PVC sizes from the original request to create the STS
			stsParams.PersistentVolumeClaim.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(test.newPVCSize)

			err = CreateOrUpdateStateFul(context.Background(), client, objMeta.Namespace, objMeta,
				stsParams, stsOwnerDef, initContainerParams, containerParams, sidecar)

			if test.expectErr != nil {
				require.Error(t, err, "Expected Error while updating Statefulset")
				require.Equal(t, test.expectErr, err)
				getUpdatedSts, err := client.AppsV1().StatefulSets(objMeta.Namespace).Get(context.Background(), objMeta.Name, metav1.GetOptions{})
				require.NoError(t, err, "Error getting StatefulSet")
				require.Equal(t, getCreatedSts, getUpdatedSts)
			} else {
				require.NoError(t, err, "Error while updating Statefulset")
				getUpdatedSts, err := client.AppsV1().StatefulSets(objMeta.Namespace).Get(context.Background(), objMeta.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if test.expectedAnnotationUpdate {
					expected := resource.MustParse(test.newPVCSize)
					require.Equal(t, strconv.FormatInt(expected.Value(), 10), getUpdatedSts.Annotations["storageCapacity"])
				}
				if test.expectedVCTUpdate {
					require.Equal(t, resource.MustParse(test.newPVCSize), getUpdatedSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage])
				} else {
					require.Equal(t, resource.MustParse(test.startingPVCSize), getUpdatedSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage])
				}
			}
		})
	}
}

func TestEnableRedisMonitoring(t *testing.T) {
	tests := []struct {
		name                  string
		redisExporterParams   containerParameters
		expectedRedisExporter corev1.Container
	}{
		{
			name: "Redis Monitoring",
			redisExporterParams: containerParameters{
				RedisExporterImage:           "redis-exporter:latest",
				RedisExporterImagePullPolicy: corev1.PullIfNotPresent,
				RedisExporterPort:            ptr.To(9121),
			},
			expectedRedisExporter: corev1.Container{
				Name:            "redis-exporter",
				Image:           "redis-exporter:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{
						Name:  "REDIS_EXPORTER_WEB_LISTEN_ADDRESS",
						Value: ":9121",
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "redis-exporter",
						ContainerPort: 9121,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			redisExporter := enableRedisMonitoring(test.redisExporterParams)
			assert.Equal(t, redisExporter, test.expectedRedisExporter, "Redis Exporter Configuration")
		})
	}
}

func TestGenerateContainerDef(t *testing.T) {
	probe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sh", "-ec", "RESP=\"$(redis-cli -h $(hostname) -p ${REDIS_PORT} ping)\"\n[ \"$RESP\" = \"PONG\" ]"},
			},
		},
	}
	tests := []struct {
		name                    string
		containerName           string
		containerDef            containerParameters
		expectedContainerDef    []corev1.Container
		redisClusterMode        bool
		containerNodeConfVolume bool
		containerEnableMetrics  bool
		containerExternalConfig *string
		redisClusterVersion     *string
		containerMountPaths     []corev1.VolumeMount
		sideCareContainer       []common.Sidecar
	}{
		{
			name:          "redis-1",
			containerName: "redis",
			containerDef: containerParameters{
				Image:              "redis:latest",
				ImagePullPolicy:    corev1.PullAlways,
				EnabledPassword:    ptr.To(false),
				PersistenceEnabled: ptr.To(false),
				AdditionalEnvVariable: &[]corev1.EnvVar{
					{
						Name:  "Add_ENV",
						Value: "Add_Value",
					},
				},
			},
			expectedContainerDef: []corev1.Container{
				{
					Name:            "redis",
					Image:           "redis:latest",
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redisVolume", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					// Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					Env: []corev1.EnvVar{
						{
							Name:  "REDIS_ADDR",
							Value: "redis://localhost:6379",
						},
						{
							Name:  "REDIS_MAJOR_VERSION",
							Value: "1.0",
						},
						{
							Name:  "SERVER_MODE",
							Value: "",
						},
						{
							Name:  "SETUP_MODE",
							Value: "",
						},
						{
							Name:  "Add_ENV",
							Value: "Add_Value",
						},
					},
					ReadinessProbe: &probe,
					LivenessProbe:  &probe,
				},
			},
			redisClusterMode:        false,
			containerNodeConfVolume: false,
			containerEnableMetrics:  false,
			containerExternalConfig: nil,
			redisClusterVersion:     ptr.To("1.0"),
			containerMountPaths:     []corev1.VolumeMount{},
			sideCareContainer:       []common.Sidecar{},
		},
		{
			name:          "redis-2",
			containerName: "redis",
			containerDef: containerParameters{
				Image:              "redis:latest",
				ImagePullPolicy:    corev1.PullAlways,
				EnabledPassword:    ptr.To(false),
				PersistenceEnabled: ptr.To(false),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			expectedContainerDef: []corev1.Container{
				{
					Name:            "redis",
					Image:           "redis:latest",
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: getVolumeMount("redisVolume", ptr.To(false), false, false, nil, []corev1.VolumeMount{
						{
							Name:      "external-config",
							ReadOnly:  false,
							MountPath: "/etc/redis/external.conf.d",
						},
					}, nil, nil),
					// Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					Env: []corev1.EnvVar{
						{
							Name:  "REDIS_ADDR",
							Value: "redis://localhost:6379",
						},
						{
							Name:  "REDIS_MAJOR_VERSION",
							Value: "1.0",
						},
						{
							Name:  "SERVER_MODE",
							Value: "",
						},
						{
							Name:  "SETUP_MODE",
							Value: "",
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					ReadinessProbe: &probe,
					LivenessProbe:  &probe,
				},
				{
					Name: "redis-exporter",
					Ports: []corev1.ContainerPort{
						{
							Name:          "redis-exporter",
							ContainerPort: 9121,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
				{
					Name:            "redis-sidecare",
					Image:           "redis-sidecar:latest",
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "REDISEXPORTER",
							Value: "ENVVALUE",
						},
					},
					Resources:    corev1.ResourceRequirements{},
					VolumeMounts: []corev1.VolumeMount{},
					Command:      []string{"/bin/bash", "-c", "/app/restore.bash"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "redis-sidecare",
							ContainerPort: 7000,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			redisClusterMode:        false,
			containerNodeConfVolume: false,
			containerEnableMetrics:  true,
			containerExternalConfig: ptr.To("some-config"),
			redisClusterVersion:     ptr.To("1.0"),
			containerMountPaths:     []corev1.VolumeMount{},
			sideCareContainer: []common.Sidecar{
				{
					Name:            "redis-sidecare",
					Image:           "redis-sidecar:latest",
					ImagePullPolicy: corev1.PullAlways,
					EnvVars: &[]corev1.EnvVar{
						{
							Name:  "REDISEXPORTER",
							Value: "ENVVALUE",
						},
					},
					Resources: &corev1.ResourceRequirements{},
					Volumes:   &[]corev1.VolumeMount{},
					Command:   []string{"/bin/bash", "-c", "/app/restore.bash"},
					Ports: &[]corev1.ContainerPort{
						{
							Name:          "redis-sidecare",
							ContainerPort: 7000,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			containerDef := generateContainerDef(test.containerName, test.containerDef, test.redisClusterMode, test.containerNodeConfVolume, test.containerEnableMetrics, test.containerExternalConfig, test.redisClusterVersion, test.containerMountPaths, test.sideCareContainer)
			assert.Equal(t, containerDef, test.expectedContainerDef, "Container Configuration")
		})
	}
}

func TestGenerateInitContainerDef(t *testing.T) {
	tests := []struct {
		name                     string
		initContainerName        string
		initContainerDef         initContainerParameters
		expectedInitContainerDef []corev1.Container
		mountPaths               []corev1.VolumeMount
	}{
		{
			name:              "Test1_With_Resources_AdditionalENV",
			initContainerName: "redis",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(false),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("220m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
				AdditionalEnvVariable: &[]corev1.EnvVar{
					{
						Name:  "TLS_MODE",
						Value: "true",
					},
				},
			},
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "initredis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redisVolume", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("220m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "TLS_MODE",
							Value: "true",
						},
					},
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
		{
			name:              "Test2_With_Volume",
			initContainerName: "redis",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(true),
			},
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "initredis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: getVolumeMount("redis", ptr.To(true), false, false, nil, []corev1.VolumeMount{
						{
							Name:      "Redis-1",
							MountPath: "/data",
						},
					}, nil, nil),
					Env: []corev1.EnvVar{},
				},
			},
			mountPaths: []corev1.VolumeMount{
				{
					Name:      "Redis-1",
					MountPath: "/data",
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			initContainer := generateInitContainerDef("", test.initContainerName, test.initContainerDef, nil, test.mountPaths, containerParameters{}, ptr.To("v6"))
			assert.Equal(t, initContainer, test.expectedInitContainerDef, "Init Container Configuration")
		})
	}
}

func TestGenerateInitContainerDefWithSecurityContext(t *testing.T) {
	// Save original feature gate state
	originalEnabled := features.Enabled(features.GenerateConfigInInitContainer)

	// Test cases for SecurityContext functionality
	tests := []struct {
		name                     string
		initContainerName        string
		initContainerDef         initContainerParameters
		featureEnabled           bool
		expectedInitContainerDef []corev1.Container
		mountPaths               []corev1.VolumeMount
	}{
		{
			name:              "Test with SecurityContext when feature is enabled",
			initContainerName: "redis",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(false),
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:  ptr.To(int64(1000)),
					RunAsGroup: ptr.To(int64(1000)),
				},
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			featureEnabled: true,
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "init-config",
					Image:           "redis-operator:latest", // This will be the operator image
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/operator", "agent"},
					Args:            []string{"bootstrap"},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(1000)),
						RunAsGroup: ptr.To(int64(1000)),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/redis",
						},
					},
				},
				{
					Name:            "initredis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redis", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(1000)),
						RunAsGroup: ptr.To(int64(1000)),
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					Env: []corev1.EnvVar{},
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
		{
			name:              "Test with SecurityContext when feature is disabled",
			initContainerName: "redis",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(false),
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:  ptr.To(int64(1000)),
					RunAsGroup: ptr.To(int64(1000)),
				},
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			featureEnabled: false,
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "initredis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redis", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(1000)),
						RunAsGroup: ptr.To(int64(1000)),
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					Env: []corev1.EnvVar{},
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
		{
			name:              "Test without SecurityContext when feature is enabled",
			initContainerName: "redis",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-init-container:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/restore.bash"},
				PersistenceEnabled: ptr.To(false),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			featureEnabled: true,
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "init-config",
					Image:           "redis-operator:latest", // This will be the operator image
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/operator", "agent"},
					Args:            []string{"bootstrap"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/redis",
						},
					},
				},
				{
					Name:            "initredis",
					Image:           "redis-init-container:latest",
					Command:         []string{"/bin/bash", "-c", "/app/restore.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("redis", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					Env: []corev1.EnvVar{},
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
		{
			name:              "Test sentinel role with SecurityContext when feature is enabled",
			initContainerName: "sentinel",
			initContainerDef: initContainerParameters{
				Enabled:            ptr.To(true),
				Image:              "redis-sentinel-init:latest",
				ImagePullPolicy:    corev1.PullAlways,
				Command:            []string{"/bin/bash", "-c", "/app/sentinel-init.bash"},
				PersistenceEnabled: ptr.To(false),
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:  ptr.To(int64(999)),
					RunAsGroup: ptr.To(int64(999)),
				},
			},
			featureEnabled: true,
			expectedInitContainerDef: []corev1.Container{
				{
					Name:            "init-config",
					Image:           "redis-operator:latest", // This will be the operator image
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/operator", "agent"},
					Args:            []string{"bootstrap", "--sentinel"},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(999)),
						RunAsGroup: ptr.To(int64(999)),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/redis",
						},
					},
				},
				{
					Name:            "initsentinel",
					Image:           "redis-sentinel-init:latest",
					Command:         []string{"/bin/bash", "-c", "/app/sentinel-init.bash"},
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts:    getVolumeMount("sentinel", ptr.To(false), false, false, nil, []corev1.VolumeMount{}, nil, nil),
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(999)),
						RunAsGroup: ptr.To(int64(999)),
					},
					Resources: corev1.ResourceRequirements{},
					Env:       []corev1.EnvVar{},
				},
			},
			mountPaths: []corev1.VolumeMount{},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			// Set feature gate state for this test
			if test.featureEnabled {
				if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=true"); err != nil {
					t.Fatalf("failed to set feature gate: %v", err)
				}
			} else {
				if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=false"); err != nil {
					t.Fatalf("failed to set feature gate: %v", err)
				}
			}

			// Note: The operator image will be determined by the actual implementation
			// We'll test the SecurityContext functionality without mocking the image

			initContainer := generateInitContainerDef("", test.initContainerName, test.initContainerDef, nil, test.mountPaths, containerParameters{}, ptr.To("v6"))

			// Verify the SecurityContext is correctly applied
			if test.featureEnabled {
				// When feature is enabled, we should have both init-config and user init container
				assert.Len(t, initContainer, 2, "Should have 2 init containers when feature is enabled")

				// Check init-config container
				initConfigContainer := initContainer[0]
				assert.Equal(t, "init-config", initConfigContainer.Name)
				assert.Equal(t, test.initContainerDef.SecurityContext, initConfigContainer.SecurityContext, "init-config container should have SecurityContext")

				// Check user init container
				userInitContainer := initContainer[1]
				assert.Equal(t, "init"+test.initContainerName, userInitContainer.Name)
				assert.Equal(t, test.initContainerDef.SecurityContext, userInitContainer.SecurityContext, "user init container should have SecurityContext")
			} else {
				// When feature is disabled, we should only have user init container
				assert.Len(t, initContainer, 1, "Should have 1 init container when feature is disabled")

				userInitContainer := initContainer[0]
				assert.Equal(t, "init"+test.initContainerName, userInitContainer.Name)
				assert.Equal(t, test.initContainerDef.SecurityContext, userInitContainer.SecurityContext, "user init container should have SecurityContext")
			}
		})
	}

	// Restore original feature gate state
	if originalEnabled {
		if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=true"); err != nil {
			t.Fatalf("failed to restore feature gate: %v", err)
		}
	} else {
		if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=false"); err != nil {
			t.Fatalf("failed to restore feature gate: %v", err)
		}
	}
}

func TestGenerateTLSEnvironmentVariables(t *testing.T) {
	tlsConfig := &common.TLSConfig{
		CaKeyFile:   "test_ca.crt",
		CertKeyFile: "test_tls.crt",
		KeyFile:     "test_tls.key",
	}

	envVars := GenerateTLSEnvironmentVariables(tlsConfig)

	expectedEnvVars := []corev1.EnvVar{
		{
			Name:  "TLS_MODE",
			Value: "true",
		},
		{
			Name:  "REDIS_TLS_CA_KEY",
			Value: path.Join("/tls/", "test_ca.crt"),
		},
		{
			Name:  "REDIS_TLS_CERT",
			Value: path.Join("/tls/", "test_tls.crt"),
		},
		{
			Name:  "REDIS_TLS_CERT_KEY",
			Value: path.Join("/tls/", "test_tls.key"),
		},
	}
	assert.ElementsMatch(t, envVars, expectedEnvVars, "EnvVars generated for TLS config are not as expected")
}

func TestGetEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                string
		role                string
		enabledPassword     *bool
		secretName          *string
		secretKey           *string
		persistenceEnabled  *bool
		tlsConfig           *common.TLSConfig
		aclConfig           *common.ACLConfig
		envVar              *[]corev1.EnvVar
		port                *int
		clusterVersion      *string
		expectedEnvironment []corev1.EnvVar
	}{
		{
			name:               "Test with role sentinel, metrics true, password true, persistence true, exporter env, tls enabled, acl enabled and env var",
			role:               "sentinel",
			enabledPassword:    ptr.To(true),
			secretName:         ptr.To("test-secret"),
			secretKey:          ptr.To("test-key"),
			persistenceEnabled: ptr.To(true),
			tlsConfig: &common.TLSConfig{
				CaKeyFile:   "test_ca.crt",
				CertKeyFile: "test_tls.crt",
				KeyFile:     "test_tls.key",
				Secret: corev1.SecretVolumeSource{
					SecretName: "tls-secret",
				},
			},
			aclConfig: &common.ACLConfig{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "acl-secret",
				},
			},
			envVar: &[]corev1.EnvVar{
				{Name: "TEST_ENV", Value: "test-value"},
			},
			clusterVersion: ptr.To("v6"),
			expectedEnvironment: []corev1.EnvVar{
				{Name: "ACL_MODE", Value: "true"},
				{Name: "PERSISTENCE_ENABLED", Value: "true"},
				{Name: "REDIS_ADDR", Value: "redis://localhost:26379"},
				{Name: "TLS_MODE", Value: "true"},
				{Name: "REDIS_TLS_CA_KEY", Value: path.Join("/tls/", "test_ca.crt")},
				{Name: "REDIS_TLS_CERT", Value: path.Join("/tls/", "test_tls.crt")},
				{Name: "REDIS_TLS_CERT_KEY", Value: path.Join("/tls/", "test_tls.key")},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "test-key",
					},
				}},
				{Name: "SERVER_MODE", Value: "sentinel"},
				{Name: "SETUP_MODE", Value: "sentinel"},
				{Name: "TEST_ENV", Value: "test-value"},
				{Name: "REDIS_MAJOR_VERSION", Value: "v6"},
			},
		},
		{
			name:               "Test with role redis, metrics false, password nil, persistence nil, exporter nil, tls nil, acl nil and nil env var",
			role:               "redis",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: nil,
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			port:               nil,
			clusterVersion:     nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "SERVER_MODE", Value: "redis"},
				{Name: "SETUP_MODE", Value: "redis"},
			},
		},
		{
			name:               "Test with role redis, metrics false, password nil, persistence false, exporter nil, tls nil, acl nil and nil env var",
			role:               "sentinel",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: ptr.To(false),
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:26379"},
				{Name: "SERVER_MODE", Value: "sentinel"},
				{Name: "SETUP_MODE", Value: "sentinel"},
			},
		},
		{
			name:               "Test with role cluster, metrics true, password true, persistence true, exporter env, tls nil, acl enabled and env var",
			role:               "cluster",
			enabledPassword:    ptr.To(true),
			secretName:         ptr.To("test-secret"),
			secretKey:          ptr.To("test-key"),
			persistenceEnabled: ptr.To(true),
			tlsConfig:          nil,
			aclConfig:          &common.ACLConfig{},
			envVar: &[]corev1.EnvVar{
				{Name: "TEST_ENV", Value: "test-value"},
			},
			port: ptr.To(6380),
			expectedEnvironment: []corev1.EnvVar{
				{Name: "ACL_MODE", Value: "true"},
				{Name: "PERSISTENCE_ENABLED", Value: "true"},
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "test-key",
					},
				}},
				{Name: "SERVER_MODE", Value: "cluster"},
				{Name: "SETUP_MODE", Value: "cluster"},
				{Name: "TEST_ENV", Value: "test-value"},
				{Name: "REDIS_PORT", Value: "6380"},
			},
		},
		{
			name:               "Test with cluster role and only metrics enabled",
			role:               "cluster",
			enabledPassword:    nil,
			secretName:         nil,
			secretKey:          nil,
			persistenceEnabled: nil,
			tlsConfig:          nil,
			aclConfig:          nil,
			envVar:             nil,
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://localhost:6379"},
				{Name: "SERVER_MODE", Value: "cluster"},
				{Name: "SETUP_MODE", Value: "cluster"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualEnvironment := getEnvironmentVariables(tt.role, tt.enabledPassword, tt.secretName,
				tt.secretKey, tt.persistenceEnabled, tt.tlsConfig, tt.aclConfig, tt.envVar, tt.port, tt.clusterVersion, nil, nil)

			assert.ElementsMatch(t, tt.expectedEnvironment, actualEnvironment)
		})
	}
}

func Test_getExporterEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                string
		params              containerParameters
		tlsConfig           *common.TLSConfig
		envVar              *[]corev1.EnvVar
		expectedEnvironment []corev1.EnvVar
	}{
		{
			name: "Test with tls enabled and env var",
			params: containerParameters{
				TLSConfig: &common.TLSConfig{
					CaKeyFile:   "test_ca.crt",
					CertKeyFile: "test_tls.crt",
					KeyFile:     "test_tls.key",
					Secret: corev1.SecretVolumeSource{
						SecretName: "tls-secret",
					},
				},
				RedisExporterEnv: &[]corev1.EnvVar{
					{Name: "TEST_ENV", Value: "test-value"},
				},
			},
			expectedEnvironment: []corev1.EnvVar{
				{Name: "REDIS_EXPORTER_TLS_CLIENT_KEY_FILE", Value: "/tls/tls.key"},
				{Name: "REDIS_EXPORTER_TLS_CLIENT_CERT_FILE", Value: "/tls/tls.crt"},
				{Name: "REDIS_EXPORTER_TLS_CA_CERT_FILE", Value: "/tls/ca.crt"},
				{Name: "REDIS_EXPORTER_SKIP_TLS_VERIFICATION", Value: "true"},
				{Name: "TEST_ENV", Value: "test-value"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualEnvironment := getExporterEnvironmentVariables(tt.params)

			assert.ElementsMatch(t, tt.expectedEnvironment, actualEnvironment)
		})
	}
}

func TestGenerateStatefulSetsDef(t *testing.T) {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sh", "-ec", "RESP=\"$(redis-cli -h $(hostname) -p ${REDIS_PORT} ping)\"\n[ \"$RESP\" = \"PONG\" ]"},
			},
		},
	}
	probeWithTLS := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sh", "-ec", "RESP=\"$(redis-cli -h $(hostname) -p ${REDIS_PORT} --tls --cert ${REDIS_TLS_CERT} --key ${REDIS_TLS_CERT_KEY} --cacert ${REDIS_TLS_CA_KEY} ping)\"\n[ \"$RESP\" = \"PONG\" ]"},
			},
		},
	}
	tests := []struct {
		name                string
		statefulSetMeta     metav1.ObjectMeta
		stsParams           statefulSetParameters
		expectedStsDef      *appsv1.StatefulSet
		stsOwnerDef         metav1.OwnerReference
		initContainerParams initContainerParameters
		containerParams     containerParameters
		sideCareContainer   []common.Sidecar
	}{
		{
			name: "Test1_With_cluster_mode_ExternalConfig_tls",
			statefulSetMeta: metav1.ObjectMeta{
				Name:      "test-sts",
				Namespace: "test-sts",
				Annotations: map[string]string{
					"redis.opstreelabs.in":       "true",
					"redis.opstreelabs.instance": "test-sts",
				},
			},
			stsOwnerDef: metav1.OwnerReference{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
				Name:       "test-sts",
			},
			stsParams: statefulSetParameters{
				Replicas:       ptr.To(int32(3)),
				ClusterMode:    true,
				NodeConfVolume: true,
				ExternalConfig: ptr.To(""),
				Tolerations: &[]corev1.Toleration{
					{
						Key:      "node.kubernetes.io/unreachable",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoExecute,
					},
				},
				ServiceAccountName: ptr.To("redis"),
			},
			expectedStsDef: &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-sts",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "StatefulSet",
							APIVersion: "apps/v1",
							Name:       "test-sts",
						},
					},
					Annotations: map[string]string{
						"redis.opstreelabs.in":       "true",
						"redis.opstreelabs.instance": "test-sts",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					ServiceName: "test-sts-headless",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
					Replicas: ptr.To(int32(3)),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"redis.opstreelabs.in":       "true",
								"redis.opstreelabs.instance": "test-sts",
							},
						},
						Spec: corev1.PodSpec{
							Tolerations: []corev1.Toleration{
								{
									Key:      "node.kubernetes.io/unreachable",
									Operator: corev1.TolerationOpExists,
									Effect:   corev1.TaintEffectNoExecute,
								},
							},
							ServiceAccountName: "redis",
							InitContainers:     []corev1.Container{},
							Containers: []corev1.Container{
								{
									Name:  "test-sts",
									Image: "redis:latest",
									Env: []corev1.EnvVar{
										{
											Name:  "ACL_MODE",
											Value: "true",
										},
										{
											Name:  "REDIS_ADDR",
											Value: "redis://localhost:6379",
										},
										{
											Name:  "REDIS_MAJOR_VERSION",
											Value: "1.0",
										},
										{
											Name:  "REDIS_TLS_CA_KEY",
											Value: path.Join("/tls/", "ca.crt"),
										},
										{
											Name:  "REDIS_TLS_CERT",
											Value: path.Join("/tls/", "tls.crt"),
										},
										{
											Name:  "REDIS_TLS_CERT_KEY",
											Value: path.Join("/tls/", "tls.key"),
										},
										{
											Name:  "SERVER_MODE",
											Value: "",
										},
										{
											Name:  "SETUP_MODE",
											Value: "",
										},
										{
											Name:  "TLS_MODE",
											Value: "true",
										},
									},
									ReadinessProbe: probeWithTLS,
									LivenessProbe:  probeWithTLS,
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "tls-certs",
											MountPath: "/tls",
											ReadOnly:  true,
										},
										{
											Name:      "acl-secret",
											MountPath: "/etc/redis/user.acl",
											SubPath:   "user.acl",
										},
										{
											Name:      "external-config",
											MountPath: "/etc/redis/external.conf.d",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "external-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{},
									},
								},
								{
									Name: "tls-certs",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "sts-secret",
										},
									},
								},
								{
									Name: "acl-secret",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "sts-acl",
										},
									},
								},
							},
						},
					},
				},
			},
			initContainerParams: initContainerParameters{},
			containerParams: containerParameters{
				Image: "redis:latest",
				EnvVars: &[]corev1.EnvVar{
					{
						Name:  "REDIS_MAJOR_VERSION",
						Value: "1.0",
					},
				},
				TLSConfig: &common.TLSConfig{
					Secret: corev1.SecretVolumeSource{
						SecretName: "sts-secret",
					},
				},
				ACLConfig: &common.ACLConfig{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "sts-acl",
					},
				},
			},
			sideCareContainer: []common.Sidecar{},
		},
		{
			name: "Test2_With_initcontainer_sidecare_enabledMetrics_enable_volume_clustermode",
			statefulSetMeta: metav1.ObjectMeta{
				Name:      "test-sts",
				Namespace: "test-sts",
				Annotations: map[string]string{
					"redis.opstreelabs.in":       "true",
					"redis.opstreelabs.instance": "test-sts",
				},
			},
			stsOwnerDef: metav1.OwnerReference{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
				Name:       "test-sts",
			},
			stsParams: statefulSetParameters{
				Replicas:       ptr.To(int32(3)),
				EnableMetrics:  true,
				ClusterMode:    true,
				NodeConfVolume: true,
				ImagePullSecrets: &[]corev1.LocalObjectReference{
					{
						Name: "redis-secret",
					},
				},
			},
			expectedStsDef: &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sts",
					Namespace: "test-sts",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "StatefulSet",
							APIVersion: "apps/v1",
							Name:       "test-sts",
						},
					},
					Annotations: map[string]string{
						"redis.opstreelabs.in":       "true",
						"redis.opstreelabs.instance": "test-sts",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					ServiceName: "test-sts-headless",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
					Replicas: ptr.To(int32(3)),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"redis.opstreelabs.in":       "true",
								"redis.opstreelabs.instance": "test-sts",
							},
						},
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{
									Env:   []corev1.EnvVar{},
									Name:  "inittest-sts",
									Image: "redis-init:latest",
								},
							},
							Containers: []corev1.Container{
								{
									Name:  "test-sts",
									Image: "redis:latest",
									Env: []corev1.EnvVar{
										{
											Name:  "PERSISTENCE_ENABLED",
											Value: "true",
										},
										{
											Name:  "REDIS_ADDR",
											Value: "redis://localhost:6379",
										},
										{
											Name:  "SERVER_MODE",
											Value: "",
										},
										{
											Name:  "SETUP_MODE",
											Value: "",
										},
									},
									ReadinessProbe: probe,
									LivenessProbe:  probe,
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "node-conf",
											MountPath: "/node-conf",
											ReadOnly:  false,
										},
										{
											Name:      "test-sts",
											MountPath: "/data",
											ReadOnly:  false,
										},
									},
								},
								{
									Name: "redis-exporter",
									Ports: []corev1.ContainerPort{
										{
											Name:          "redis-exporter",
											ContainerPort: 9121,
											Protocol:      corev1.ProtocolTCP,
										},
									},
								},
								{
									Name:            "redis-sidecare",
									Image:           "redis-sidecar:latest",
									ImagePullPolicy: corev1.PullAlways,
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "additional-vol",
								},
							},
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: "redis-secret",
								},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-conf",
								Annotations: map[string]string{
									"redis.opstreelabs.in":       "true",
									"redis.opstreelabs.instance": "test-sts",
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								VolumeMode: ptr.To(corev1.PersistentVolumeFilesystem),
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-sts",
								Annotations: map[string]string{
									"redis.opstreelabs.in":       "true",
									"redis.opstreelabs.instance": "test-sts",
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								VolumeMode: ptr.To(corev1.PersistentVolumeFilesystem),
							},
						},
					},
				},
			},
			initContainerParams: initContainerParameters{
				Enabled: ptr.To(true),
				Image:   "redis-init:latest",
			},
			containerParams: containerParameters{
				Image:              "redis:latest",
				PersistenceEnabled: ptr.To(true),
				AdditionalVolume: []corev1.Volume{
					{
						Name: "additional-vol",
					},
				},
			},
			sideCareContainer: []common.Sidecar{
				{
					Name:            "redis-sidecare",
					Image:           "redis-sidecar:latest",
					ImagePullPolicy: corev1.PullAlways,
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			stsDef := generateStatefulSetsDef(test.statefulSetMeta, test.stsParams, test.stsOwnerDef, test.initContainerParams, test.containerParams, test.sideCareContainer)
			assert.Equal(t, stsDef, test.expectedStsDef, "StatefulSet Configuration")
		})
	}
}

func TestGetSidecars(t *testing.T) {
	tests := []struct {
		name            string
		sideCars        *[]common.Sidecar
		expectedSidecar []common.Sidecar
	}{
		{
			name: "TEST1_Present",
			sideCars: &[]common.Sidecar{
				{
					Command: []string{"sh", "-c", "redis-cli -h $(hostname) -p ${REDIS_PORT} ping"},
				},
			},
			expectedSidecar: []common.Sidecar{
				{
					Command: []string{"sh", "-c", "redis-cli -h $(hostname) -p ${REDIS_PORT} ping"},
				},
			},
		},
		{
			name:            "TEST2_Not_Present",
			sideCars:        &[]common.Sidecar{},
			expectedSidecar: []common.Sidecar{},
		},
		{
			name:            "TEST2_Nil",
			sideCars:        nil,
			expectedSidecar: []common.Sidecar{},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			result := getSidecars(test.sideCars)
			assert.Equal(t, test.expectedSidecar, result)
		})
	}
}

func TestStatefulSetSelectorLabels(t *testing.T) {
	tests := []struct {
		name                   string
		inputLabels            map[string]string
		expectedSelectorLabels map[string]string
	}{
		{
			name: "helm-labels-filtered-out",
			inputLabels: map[string]string{
				"app":                        "redis-replication",
				"redis_setup_type":           "replication",
				"role":                       "replication",
				"helm.sh/chart":              "redis-replication-4.5.6",
				"app.kubernetes.io/version":  "v7.0.12",
				"app.kubernetes.io/instance": "my-redis",
			},
			expectedSelectorLabels: map[string]string{
				"app":              "redis-replication",
				"redis_setup_type": "replication",
				"role":             "replication",
			},
		},
		{
			name: "only-stable-labels-present",
			inputLabels: map[string]string{
				"app":              "redis-cluster",
				"redis_setup_type": "cluster",
				"role":             "leader",
			},
			expectedSelectorLabels: map[string]string{
				"app":              "redis-cluster",
				"redis_setup_type": "cluster",
				"role":             "leader",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test StatefulSet metadata
			stsMeta := metav1.ObjectMeta{
				Name:      "test-sts",
				Namespace: "test-ns",
				Labels:    tt.inputLabels,
			}

			// Call our StatefulSet generation function
			selectorLabels := extractStatefulSetSelectorLabels(stsMeta.GetLabels())

			// Verify that the generated selector labels are correct
			assert.Equal(t, tt.expectedSelectorLabels, selectorLabels, "StatefulSet selector labels should be filtered correctly")
		})
	}
}
