package k8sutils

import (
	"reflect"
	"testing"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func Test_generateMetaInformation(t *testing.T) {
	type args struct {
		resourceKind string
		apiVersion   string
	}
	tests := []struct {
		name string
		args args
		want metav1.TypeMeta
	}{
		{"pod-generateMetaInformation", args{resourceKind: "Pod", apiVersion: "v1"}, metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateMetaInformation(tt.args.resourceKind, tt.args.apiVersion); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateMetaInformation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateObjectMetaInformation(t *testing.T) {
	type args struct {
		name        string
		namespace   string
		labels      map[string]string
		annotations map[string]string
	}
	tests := []struct {
		name string
		args args
		want metav1.ObjectMeta
	}{
		{
			name: "generateObjectMetaInformation",
			args: args{
				name:        "test",
				namespace:   "default",
				labels:      map[string]string{"test": "test"},
				annotations: map[string]string{"test": "test"},
			},
			want: metav1.ObjectMeta{
				Name:        "test",
				Namespace:   "default",
				Labels:      map[string]string{"test": "test"},
				Annotations: map[string]string{"test": "test"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateObjectMetaInformation(tt.args.name, tt.args.namespace, tt.args.labels, tt.args.annotations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateObjectMetaInformation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddOwnerRefToObject(t *testing.T) {
	type args struct {
		obj      metav1.Object
		ownerRef metav1.OwnerReference
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "AddOwnerRefToObject",
			args: args{
				obj: &metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "default",
				},
				ownerRef: metav1.OwnerReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "deployment",
					UID:        "1234567890",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddOwnerRefToObject(tt.args.obj, tt.args.ownerRef)
		})
	}
}

func Test_generateStatefulSetsAnots(t *testing.T) {
	type args struct {
		stsMeta metav1.ObjectMeta
		ignore  []string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "generateStatefulSetsAnots",
			args: args{
				stsMeta: metav1.ObjectMeta{
					Name:      "sts",
					Namespace: "default",
					Annotations: map[string]string{
						"app.kubernetes.io/name":       "redis",
						"operator.redis.com/redis":     "redis",
						"operator.redis.com/redis-uid": "1234567890",
					},
				},
			},
			want: map[string]string{
				"app.kubernetes.io/name":       "redis",
				"operator.redis.com/redis":     "redis",
				"operator.redis.com/redis-uid": "1234567890",
				"redis.opstreelabs.in":         "true",
				"redis.opstreelabs.instance":   "sts",
			},
		},
		{
			name: "generateStatefulSetsAnots_with_ignore",
			args: args{
				stsMeta: metav1.ObjectMeta{
					Name:      "sts",
					Namespace: "default",
					Annotations: map[string]string{
						"app.kubernetes.io/name":       "redis",
						"operator.redis.com/redis":     "redis",
						"operator.redis.com/redis-uid": "1234567890",
						"a":                            "b",
					},
				},
				ignore: []string{"a"},
			},
			want: map[string]string{
				"app.kubernetes.io/name":       "redis",
				"operator.redis.com/redis":     "redis",
				"operator.redis.com/redis-uid": "1234567890",
				"redis.opstreelabs.in":         "true",
				"redis.opstreelabs.instance":   "sts",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateStatefulSetsAnots(tt.args.stsMeta, tt.args.ignore); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateStatefulSetsAnots() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_filterAnnotations(t *testing.T) {
	type args struct {
		anots map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "filterAnnotations",
			args: args{
				anots: map[string]string{
					"app.kubernetes.io/name":                           "redis",
					"operator.redis.com/redis":                         "redis",
					"kubectl.kubernetes.io/last-applied-configuration": "eyJhcGlWZXJzaW9uIjoiYXBwcy92",
					"banzaicloud.com/last-applied":                     "eyJhcGlWZXJzaW9uIjoiYXBwcy92",
				},
			},
			want: map[string]string{
				"app.kubernetes.io/name":   "redis",
				"operator.redis.com/redis": "redis",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterAnnotations(tt.args.anots); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateServiceAnots(t *testing.T) {
	stsMeta := metav1.ObjectMeta{
		Name:        "test-sts",
		Annotations: map[string]string{"custom-annotation": "custom-value"},
	}

	additionalSvcAnnotations := map[string]string{
		"additional-annotation": "additional-value",
	}

	expectedAnnotations := map[string]string{
		"redis.opstreelabs.in":       "true",
		"redis.opstreelabs.instance": "test-sts",
		"prometheus.io/scrape":       "true",
		"prometheus.io/port":         "9121",
		"custom-annotation":          "custom-value",
		"additional-annotation":      "additional-value",
	}

	resultAnnotations := generateServiceAnots(stsMeta, additionalSvcAnnotations, defaultExporterPortProvider)

	if !reflect.DeepEqual(resultAnnotations, expectedAnnotations) {
		t.Errorf("Expected annotations to be %v but got %v", expectedAnnotations, resultAnnotations)
	}
}

func TestGetRedisLabels(t *testing.T) {
	tests := []struct {
		name      string
		setupType setupType
		role      string
		input     map[string]string
		expected  map[string]string
	}{
		{
			name:      "test-redis",
			setupType: cluster,
			role:      "master",
			input:     map[string]string{"custom-label": "custom-value"},
			expected: map[string]string{
				"app":              "test-redis",
				"redis_setup_type": string(cluster),
				"role":             "master",
				"custom-label":     "custom-value",
			},
		},
		{
			name:      "test-redis",
			setupType: standalone,
			role:      "master",
			input:     map[string]string{},
			expected: map[string]string{
				"app":              "test-redis",
				"redis_setup_type": string(standalone),
				"role":             "master",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRedisLabels(tt.name, tt.setupType, tt.role, tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("For name=%s, setupType=%s, and role=%s; expected %v but got %v", tt.name, tt.setupType, tt.role, tt.expected, result)
			}
		})
	}
}

func TestRedisAsOwner(t *testing.T) {
	redisObj := &redisv1beta2.Redis{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redisv1beta2/apiVersion",
			Kind:       "Redis",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-redis",
			UID:  "12345",
		},
	}

	expectedOwnerReference := metav1.OwnerReference{
		APIVersion: "redisv1beta2/apiVersion",
		Kind:       "Redis",
		Name:       "test-redis",
		UID:        "12345",
		Controller: func() *bool {
			b := true
			return &b
		}(),
	}

	result := redisAsOwner(redisObj)

	if !reflect.DeepEqual(result, expectedOwnerReference) {
		t.Errorf("Expected %v but got %v", expectedOwnerReference, result)
	}
}

func TestRedisClusterAsOwner(t *testing.T) {
	clusterObj := &redisv1beta2.RedisCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redisv1beta2/apiVersion",
			Kind:       "RedisCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-redis-cluster",
			UID:  "abcdef",
		},
	}

	expectedOwnerReference := metav1.OwnerReference{
		APIVersion: "redisv1beta2/apiVersion",
		Kind:       "RedisCluster",
		Name:       "test-redis-cluster",
		UID:        "abcdef",
		Controller: ptr.To(true),
	}

	result := redisClusterAsOwner(clusterObj)

	if !reflect.DeepEqual(result, expectedOwnerReference) {
		t.Errorf("Expected %v but got %v", expectedOwnerReference, result)
	}
}

func TestRedisReplicationAsOwner(t *testing.T) {
	replicationObj := &redisv1beta2.RedisReplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redisv1beta2/apiVersion",
			Kind:       "RedisReplication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-redis-replication",
			UID:  "ghijkl",
		},
	}

	expectedOwnerReference := metav1.OwnerReference{
		APIVersion: "redisv1beta2/apiVersion",
		Kind:       "RedisReplication",
		Name:       "test-redis-replication",
		UID:        "ghijkl",
		Controller: ptr.To(true),
	}

	result := redisReplicationAsOwner(replicationObj)

	if !reflect.DeepEqual(result, expectedOwnerReference) {
		t.Errorf("Expected %v but got %v", expectedOwnerReference, result)
	}
}

func TestRedisSentinelAsOwner(t *testing.T) {
	sentinelObj := &redisv1beta2.RedisSentinel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "redisv1beta2/apiVersion",
			Kind:       "RedisSentinel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-redis-sentinel",
			UID:  "mnopqr",
		},
	}

	expectedOwnerReference := metav1.OwnerReference{
		APIVersion: "redisv1beta2/apiVersion",
		Kind:       "RedisSentinel",
		Name:       "test-redis-sentinel",
		UID:        "mnopqr",
		Controller: ptr.To(true),
	}

	result := redisSentinelAsOwner(sentinelObj)

	if !reflect.DeepEqual(result, expectedOwnerReference) {
		t.Errorf("Expected %v but got %v", expectedOwnerReference, result)
	}
}
