package status

type RedisClusterState string

const (
	ReadyClusterReason                string = "RedisCluster is ready"
	InitializingClusterLeaderReason   string = "RedisCluster is initializing leaders"
	InitializingClusterFollowerReason string = "RedisCluster is initializing followers"
	BootstrapClusterReason            string = "RedisCluster is bootstrapping"
)

// Status Field of the Redis Cluster
const (
	RedisClusterReady        RedisClusterState = "Ready"
	RedisClusterInitializing RedisClusterState = "Initializing"
	RedisClusterBootstrap    RedisClusterState = "Bootstrap"
	// RedisClusterFailed       RedisClusterState = "Failed"
)
