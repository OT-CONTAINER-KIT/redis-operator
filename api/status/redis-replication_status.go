package status

type RedisReplicationState string

const (
	ReadyReplicationReason        string = "RedisReplication is ready"
	InitializingReplicationReason string = "RedisReplication is initializing"
	BootstrapReplicationReason    string = "RedisReplication is bootstrapping"
)

// Status Field of the Redis Replication
const (
	RedisReplicationReady        RedisReplicationState = "Ready"
	RedisReplicationInitializing RedisReplicationState = "Initializing"
	RedisReplicationBootstrap    RedisReplicationState = "Bootstrap"
	// RedisReplicationFailed       RedisReplicationState = "Failed"
)
