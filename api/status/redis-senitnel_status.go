package status

type RedisSenitnelState string

const (
	ReadySenitnelReason                string = "RedisSenitnel is ready"
	BootstrapSenitnelReason            string = "RedisSenitnel is bootstrapping"
)

// Status Field of the Redis Senitnel
const (
	RedisSenitnelReady        RedisSenitnelState = "Ready"
	RedisSenitnelInitializing RedisSenitnelState = "Initializing"
	RedisSenitnelBootstrap    RedisSenitnelState = "Bootstrap"
	// RedisSenitnelFailed       RedisSenitnelState = "Failed"
)
