package api

// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=rediss;redisclusters;redisreplications;redis;rediscluster;redissentinel;redissentinels;redisreplication,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:urls=*,verbs=get
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/finalizers;rediscluster/finalizers;redissentinel/finalizers;redisreplication/finalizers,verbs=update
// +kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/status;rediscluster/status;redissentinel/status;redisreplication/status,verbs=get;patch;update
// +kubebuilder:rbac:groups=,resources=secrets;pods/exec;pods;services;configmaps;events;persistentvolumeclaims;namespaces,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=create;delete;get;list;patch;update;watch
