package status

type RedisClusterState string

const (
	ReadyClusterReason                string = "RedisCluster is ready"
	InitializingClusterLeaderReason   string = "RedisCluster is initializing leaders"
	InitializingClusterFollowerReason string = "RedisCluster is initializing followers"
	BootstrapClusterReason            string = "RedisCluster is bootstrapping"
	ScalingDownLeaderClusterReason    string = "RedisCluster is scaling down leaders"
	ScalingDownFollowerClusterReason  string = "RedisCluster is scaling down followers"
	ReshardingClusterReason           string = "RedisCluster is resharding"
	RebalanceClusterReason            string = "RedisCluster is rebalancing"
)

// Status Field of the Redis Cluster
const (
	RedisClusterReady        RedisClusterState = "Ready"
	RedisClusterInitializing RedisClusterState = "Initializing"
	RedisClusterBootstrap    RedisClusterState = "Bootstrap"
	RedisClusterScalingDown  RedisClusterState = "ScalingDown"
	RedisClusterResharding   RedisClusterState = "Resharding"
	RedisClusterRebalancing  RedisClusterState = "Rebalancing"
	// RedisClusterFailed       RedisClusterState = "Failed"
)
