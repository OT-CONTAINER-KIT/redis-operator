package status

type RedisStandaloneState string

const (
	ReadyStandaloneReason        string = "RedisStandalone is ready"
	InitializingStandaloneReason string = "RedisStandalone is initializing"
	BootstrapStandaloneReason    string = "RedisStandalone is bootstrapping"
)

// Status Field of the Redis Standalone
const (
	RedisStandaloneReady        RedisStandaloneState = "Ready"
	RedisStandaloneInitializing RedisStandaloneState = "Initializing"
	RedisStandaloneBootstrap    RedisStandaloneState = "Bootstrap"
	// RedisStandaloneFailed       RedisStandaloneState = "Failed"
)
