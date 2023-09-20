package status

type RedisStandaloneState string

const (
	ReadyStandaloneReason        string = "RedisStandalone is ready"
	InitializingStandaloneReason string = "RedisStandalone is initializing"
)

// Status Field of the Redis Standalone
const (
	RedisStandaloneReady        RedisStandaloneState = "Ready"
	RedisStandaloneInitializing RedisStandaloneState = "Initializing"
	// RedisStandaloneFailed       RedisStandaloneState = "Failed"
)
