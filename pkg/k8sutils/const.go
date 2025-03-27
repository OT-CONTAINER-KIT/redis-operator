package k8sutils

const (
	OperatorImage = "quay.io/opstree/redis-operator:v0.19.1"
)

const (
	AnnotationKeyRecreateStatefulset         = "redis.opstreelabs.in/recreate-statefulset"
	AnnotationKeyRecreateStatefulsetStrategy = "redis.opstreelabs.in/recreate-statefulset-strategy"
)

const (
	EnvOperatorSTSPVCTemplateName = "OPERATOR_STS_PVC_TEMPLATE_NAME"
)

const (
	RedisRoleLabelKey    = "redis-role"
	RedisRoleLabelMaster = "master"
	RedisRoleLabelSlave  = "slave"
)

const (
	VolumeNameConfig = "config"
)
