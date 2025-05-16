package status

type RedisClusterState string

const (
	InitializingClusterLeaderReason   string = "RedisCluster is initializing leaders"
	InitializingClusterFollowerReason string = "RedisCluster is initializing followers"
	BootstrapClusterReason            string = "RedisCluster is bootstrapping"
	ReadyClusterReason                string = "RedisCluster is ready"
)

// Status Field of the Redis Cluster
const (
	RedisClusterInitializing RedisClusterState = "Initializing"
	RedisClusterBootstrap    RedisClusterState = "Bootstrap"
	// RedisClusterReady means the RedisCluster is ready for use, we use redis-cli --cluster check 127.0.0.1:6379 to check the cluster status
	RedisClusterReady  RedisClusterState = "Ready"
	RedisClusterFailed RedisClusterState = "Failed"
)
