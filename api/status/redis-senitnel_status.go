package status

type RedisSentinelState string

const (
	ReadySentinelReason        string = "RedisSenitnel is ready"
	InitializingSentinelReason string = "RedisSentinel is initializing"
	BootstrapSentinelReason    string = "RedisSenitnel is bootstrapping"
)

// Status Field of the Redis Senitnel
const (
	RedisSenitnelReady        RedisSentinelState = "Ready"
	RedisSentinelInitializing RedisSentinelState = "Initializing"
	RedisSentinelBootstrap    RedisSentinelState = "Bootstrap"
)
