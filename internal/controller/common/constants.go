package common

const (
	AnnotationKeyRecreateStatefulset         = "redis.opstreelabs.in/recreate-statefulset"
	AnnotationKeyRecreateStatefulsetStrategy = "redis.opstreelabs.in/recreate-statefulset-strategy"
	// AnnotationKeyExternalConfigChecksum is set on the pod template and holds a
	// checksum of the mounted external/additional config ConfigMap. Because
	// Kubernetes does not restart pods when a mounted ConfigMap changes, encoding
	// the content into the pod template makes the StatefulSet spec change whenever
	// the config changes, which triggers a rolling update.
	AnnotationKeyExternalConfigChecksum = "redis.opstreelabs.in/external-config-checksum"
	// AnnotationKeyTLSSecretChecksum, AnnotationKeyACLSecretChecksum and
	// AnnotationKeyPasswordSecretChecksum do the same for the user-provided Secrets
	// the operator mounts or references (TLS certificates, ACL rules, and the
	// password injected via secretKeyRef). Rotating any of them rolls the pods so
	// the new material is actually picked up.
	AnnotationKeyTLSSecretChecksum      = "redis.opstreelabs.in/tls-secret-checksum"
	AnnotationKeyACLSecretChecksum      = "redis.opstreelabs.in/acl-secret-checksum"
	AnnotationKeyPasswordSecretChecksum = "redis.opstreelabs.in/password-secret-checksum"
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
