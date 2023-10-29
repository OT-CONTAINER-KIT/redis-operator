package status

type RedisClusterState string

const (
	ReadyClusterReason                string = "RedisCluster is ready"
	InitializingClusterLeaderReason   string = "RedisCluster is initializing leaders"
	InitializingClusterFollowerReason string = "RedisCluster is initializing followers"
	BootstrapClusterReason            string = "RedisCluster is bootstrapping"
	ScalingUpLeaderClusterReason      string = "RedisCluster is scaling up leaders"
	ScalingUpFollowerClusterReason    string = "RedisCluster is scaling up followers"
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
	RedisClusterScalingUp    RedisClusterState = "ScalingUp"
	RedisClusterScalingDown  RedisClusterState = "ScalingDown"
	RedisClusterResharding   RedisClusterState = "Resharding"
	RedisClusterRebalancing  RedisClusterState = "Rebalancing"
	// RedisClusterFailed       RedisClusterState = "Failed"
)
