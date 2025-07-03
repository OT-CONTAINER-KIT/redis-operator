package common

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

const (
	RedisPort             = 6379
	SentinelPort          = 26379
	RedisExporterPort     = 9121
	RedisExporterPortName = "redis-exporter"
)
