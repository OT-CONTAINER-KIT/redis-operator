package k8sutils

const (
	AnnotationKeyRecreateStatefulset = "redis.opstreelabs.in/recreate-statefulset"
)

const (
	EnvOperatorSTSPVCTemplateName = "OPERATOR_STS_PVC_TEMPLATE_NAME"
)

const (
	RedisRoleLabelKey    = "redis-role"
	RedisRoleLabelMaster = "master"
	RedisRoleLabelSlave  = "slave"
)
