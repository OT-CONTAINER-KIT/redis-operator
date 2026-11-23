package common

// SentinelStartupAnnotation is activated only after a live Sentinel majority
// has been confirmed. It remains set while routing is temporarily unavailable.
const SentinelStartupAnnotation = "redis.opstreelabs.in/sentinel-startup"

// SentinelOwnershipAnnotation records the irreversible role-ownership handoff
// before any monitor is created, independently of successful routing publication.
const SentinelOwnershipAnnotation = "redis.opstreelabs.in/sentinel-ownership"

// SentinelReplacementRecoveryAnnotation is controller-only recovery bookkeeping.
// It must never enter workload templates and trigger an unrelated rolling update.
const SentinelReplacementRecoveryAnnotation = "redis.opstreelabs.in/sentinel-replacement-recovery"

const VolumeNameSentinelState = "sentinel-state"
