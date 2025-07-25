### v0.21.0
##### June 2025 Latest

#### :tada: Features

- Round robin where to transfer cluster shards when scaling in a Redis Cluster #1412
- Add auto max memory configuration for Redis instances #1411
- Add bus port configuration for Redis cluster services #1406
- Add automatic Redis pod role label synchronization for rediscluster #1404
- RedisCluster observability #1392
- Add liveness/readiness probes to values.yaml and templates #1378
- Reduce unnecessary requeue when skip reconcile annotation exists #1374
- RedisReplication observability, skip reconcile or not #1369
- Add Redis Sentinel validation webhook for clusterSize #1361

#### :beetle: Bug Fixes

- Resolve StatefulSet selector immutability issues #1382
- Avoid sentinel restart after replication failover #1381
- Define named probe port outside webhook block #1354

#### :tada: Refactors

- Reorganize manager agent cmd package #1383
- Reorganize API structure and update paths #1363
- Remove useless structure and refactor package #1362
- Reorganize command structure for Redis operator #1351

### v0.20.2
##### May 12, 2024

#### :beetle: Bug Fixes

- Handle panic when retrieving StatefulSet in GetRedisNodesByRole #1330
- VCT resize detection logic; add support for scaling out with new VCT size #1342
- Service updated before Statefulset during Reconciliation #1348

#### :tada: Features

- Add data assertion generation and enhance Redis configuration commands #1331
- Add feature gates support for Redis Operator #1333
- Migrate kubebuilder go.kubebuilder.io/v3 to go.kubebuilder.io/v4 #1340

#### :tada: Refactors

- Define container port for http probes in operator chart #1326
- Enhance environment variable management and CI workflow #1315

### v0.20.1
##### April 27, 2024

#### :beetle: Bug Fixes

- Move VCT logic before diff calculation for stateful set #1322

### v0.20.0
##### April 1, 2024

#### :tada: Features

- Sentinel - support hostname resolve and announce #1247
- Add redis agent with bootstrap configuration generation #1254
- Implement comprehensive Redis configuration generation for agent bootstrap #1260
- Added support for hostport to allow direct connection to the pod #1263
- Add preStop hook for Redis Cluster failover #1264
- Sentinel - announce-ip when resolve & announce are set #1271
- Fix PVC resizing issue and refactor PVC resizing logic #1268
- Add redisreplication observability #1274
- Add recreate-stateful-strategy, orphan, background, foreground(default) #1286
- Update Dockerfile and Makefile for unified operator binary #1294
- Add support for anti affinity configuration in helm charts #1296
- Guarantee to avoid bad master ip on Sentinel #1289
- Add feature gates for sentinel configuration generation in init container #1300
- Support redis configuration generation in init container #1303

#### :beetle: Bug Fixes

- Replace hardcoded Redis port 6379 with configurable port from cr.Spec.Port #1261
- Update references from master to main in docs and workflow files #1288
- Svc finalizer removed #1297
- Race condition resulting in permanently broken Redis cluster #1298

### v0.19.1
##### February 19, 2024

#### :warning: Deprecation Notice

The v1beta1 API version will be removed in next release. Users are strongly encouraged to migrate to v1beta2, which offers enhanced features and improved stability.

#### :tada: Features

- Add data-assert tool for Redis data management #1204
- Check data consistent by external tool #1205
- Added actions to publish charts to github container registry #1201
- Add headless service configuration support #1219
- Update redis-operator cert manager configuration #1220
- Add additional service configuration with optional enable flag #1228
- Enhance Redis HA and node scheduling strategy #1237
- Add securityContext config in chart for redis-exporter #1238
- Add dynamic Redis configuration support for Redis Cluster #1241
- Configurable operator maxConcurrentReconciles #1242

#### :beetle: Bug Fixes

- Skip-reconcile annotation still skipping reconcile even the value is false #1202
- Make recreate statefulset only trigger when value true #1240
- Improve pod label patching with error handling and retry mechanism #1231
- Changed certificate serverName to pod+namespace #1221
- Add missing topologySpreadConstraint on RedisCluster follower #1218

### v0.19.0
##### January 12, 2024

#### :tada: Features

- Add PDB and probes, drop unspecified acl in sentinel helm #1123
- Add master/replica service to redis replication #1124
- Add recreateStatefulSetOnUpdateInvalid helm chart value #1127
- Enhance RedisReplication controller and CRD with additional status #1154
- Support PDB in redisreplication #1166
- Enhance RedisSentinel reconciliation logic and update workflow #1176
- Support redis-cluster topologySpreadConstraints #1177
- Add event recording functionality for RedisCluster controller #1182
- Support topologySpreadConstraints in replication & sentinel #1184
- Redis-cluster add podAntiAffinity #1180
- Separate resources section for leader and follower #1188
- Enhance RedisCluster resource management by introducing separate resource handling for leader and follower #1199

#### :beetle: Bug Fixes

- PDB value mapping in redis-sentinel #1136
- Chart render error when enable initcontainer #1146
- InitContainer enabled properties not define in template #1152
- Redis-cluster unexpected downscaling #1173
- Reduce the impact of Redis cluster intermediate states #1178
- Label selector mapping on redisreplication pdb #1191

### v0.18.1
##### November 7, 2024

#### :tada: Features

- Support setting minReadySeconds on the stateful sets #1023
- Add tolerations to operator chart #1051
- Add image pull secret for redis operator #1053
- Add service monitor to redis sentinel chart #1071
- Add readiness/liveness probe to redis operator chart #1072
- Upgrade redis/sentinel image to 7.0.15 #1099
- Reconcile redissentinel only on master changed #1122

#### :beetle: Bug Fixes

- Fix indentation error when enable additional config #1031
- Fix field validate error when enable additional config for sentinel #1033
- Fix unknown field error when upgrade chart #1034
- Fix bad indentation on redis standalone additional configs #1040
- Fix attempt to repair disconnected/failed master nodes before failing over #1105
- Fix set controller probe endpoint handler #1121

### v0.18.0
##### July 11, 2024

#### :tada: Features

- Added redisReplicationPassword values to connect secured replication #1021
- Added redis/redisreplication/redissentinel/rediscluster chart #1007
- Added support for extra volume mounts for redis sentinel #994
- Added automountServiceAccountToken values for deployment and serviceaccount #991
- Added securityContext for exporter, initcontainers and sidecars #987
- Added security context values in operator chart #973
- Added rolling update sequence from leader to follower #966
- Added support for configurable probe handlers #934
- Added redis operator helm chart and release workflow #941
- Added support for other container engines #947

#### :beetle: Bug Fixes

- Added default port to enable `SENTINEL_PORT` environment #999
- ReadyReplicas need to be checked in `IsStatefulSetReady` #993
- watchNamespace value does not take effect in chart #990
- Sentinel should not reconcile until replication cluster ready #964
- Return ASAP after handling finalizer #940
- Check redis replication after handling finalizer #936

### v0.17.0
##### May 14, 2024

#### :tada: Features

- WATCH_NAMESPACE support multi namespace #919
- Add workflow to publish image to ghcr #914
- Probe use built-in, discarded healthcheck.sh #907
- Implement redis cluster ready state #867
- Add redisreplication status masterNode #849

#### :beetle: Bug Fixes

- Runtime panic when delete rediscluster which disable persistent #922
- Runtime panic when storage param is empty #887
- Exporter can not connect to redis when enable tls #902
- Update status if not equal #900
- Should get the really leader count when scale in #885
- ClusterSlaves result should be cut #884
- Redis cluster update as scale out #882
- Add common AddFinalizer for all api #858

### v0.16.0
##### March 27, 2024

#### :tada: Features

- Added support for multiple CRDs and enhanced functionality
- Improved test coverage and CI/CD pipeline
- Enhanced Redis cluster management and scaling capabilities
- Added support for custom Redis configurations
- Implemented advanced security features

#### :beetle: Bug Fixes

- Fixed various StatefulSet and service management issues
- Resolved Redis cluster scaling problems
- Fixed authentication and TLS connectivity issues
- Improved error handling and logging

### v0.15.0 
##### July 17, 2023

#### :beetle: Bug Fixes

- Fix Linter Issue #479 
- Fix exporter ports enabled even when exporters disabled #484
- Corrected scenario "go-get-tool" in makefile #499
- Operator Crash when persistence is false/Disabled #519 -- Breaking Change
- call of func checkAttachedSlave for 1 Master Replication #523
- Only created /node-conf VolumeMount for clusters #532
- Redis Sentinel Exporter ports in Env Vars #533
- Init Container tried to mount invalid volume name #538
- Cluster leader failover loop if there is only a single leader #542

#### :tada: Features

- Add RedisExporter for sentinel #440
- Add InitContainer Field #458
- ACL redis via secret #486
- Adding Custom TerminationGracePeriodSeconds and additional fields for Sidecar #487
- Enable Support for Backup and Restore via script #489
- Support Scaling for Redis Cluster #531 -- Breaking Change

#### :tada: Refactors

- Fixed StatefulSet(sentinel) Label for Service(Selector) #442
- Declare Module Correctly On sentinel #478
- Manage (Pod and Container) security Context explicitly #518
- Add watchnamespace function as per operator hub #520
- Remove sentinel default validation not effect #535
- Remove sentinel cluster size validation no effect #536

### v0.14.0 
##### Feburary 13, 2023

#### :beetle: Bug Fixes

- Added check for persistent volume nil condition
- Fix crash with go panic
- Fix memory address bug and nil pointer
- CR annotations fixes w.r.t. to stateful set
- Fix issues with ARM64 support

#### :tada: Features

- Added serviceType functionality for redis standalone and cluster
- Added feature for additional volume mounts
- Added nodeSelectory and tolerations for redis cluster
- Added recreation logic for redis stateful sets
- Added replication mode support for the redis cluster
- Added sentinel support for replication failover

### v0.13.0
##### November 10, 2022

#### :beetle: Bug Fixes

- Fixed multiple follower logic for redis cluster
- Fixed CR's annotations updated,sts annotations will not updated

#### :tada: Features

- Updated all examples for Redis v7
- Revamped documentation with the latest information
- Added pause option for reconcilations
- Added support for arm64
- Added update strategy for statefulset
- Added logic for updating follower replicas
- Added TLS feature for standalone

### v0.12.0
##### October 12, 2022

#### :beetle: Bug Fixes

- PDB (Pod disruption budget) creation issue
- Fixed cluster recovery logic
- Fixed IP check and conversion logic
- Persistence issue fix

#### :tada: Features

- Added pvc, pv clusterrole fix
- Support for defining serviceAccount
- Closing of redis client connection
- Added finalizer for statefulset
- Added Prometheus service annotation
- Added support for Redis 7 with DNS hostname

### v0.11.0
##### July 5, 2022

#### :beetle: Bug Fixes

- Fix Redis cluster and Redis CRD
- Fixed TLS authentication between redis cluster
- Fixed RBAC policy for PDB
- Redis exporter exception handled
- External service fix

### v0.10.0
##### January 26, 2022

#### :tada: Features

- Added custom probes capability
- Added sidecar support for redis
- Added option for namespaced operator
- Added finalizers for Kubernetes resources
- Adding PodDisruptionBudget support
- Added TLS cluster support
- Pass through Annotations and Labels to all Child resources
- Adding Rudimentry IPv6 Support

#### :beetle: Bug Fixes

- Fix up RedisClusterStatus Spec being incorrect object
- Fixed invalid RBAC kustomization
- Fixed RBAC role for operator
- Fixed service creation for leader and follower
- 
### v0.9.0
##### November 13, 2021

#### :tada: Features

- Added RBAC policies for redis operator with least privileges

#### :beetle: Bug Fixes

- Fix and updated documentation dependencies
- Test pointers before dereferencing
- Fix panic error of golang for redis exporter
- Fix resource block nil exception for redis exporter

### v0.8.0
##### September 3, 2021

#### :tada: Features

- Added external configuration capability for follower and leader
- Streamlined examples folder with different examples for standalone and cluster
- Added the capability for affinity for leader and follower

### v0.7.0
##### August 12, 2021

#### :tada: Features

- Remove all the vulnerable dependencies from docs(NodeJS)
- Added a new grafana dashboard for better monitoring visualization
- Added environment variable support for redis exporter
- Added Image Pull Secret support for private registeries

#### :beetle: Bug Fixes

- Fix bug for non-defined storage
- Fixed secret nil exception bug
- Fixed bug for making redis exporter optional

### v0.6.0
##### June 11, 2021

#### :tada: Features

- Breaked the CRDs into Redis standalone cluster setup
- Optimized code configuration for creating Redis cluster
- Removed string secret type and secret type password is only supported
- Structured and optimized golang based codebase
- Removed divisive terminlogies

#### :beetle: Bug Fixes

- Removed the monitor label to resolve service endpoint issue

### v0.5.0
##### May 1, 2021

#### :tada: Features

- Added support for recovering redis nodes from failover
- Added toleration support for redis statefuls
- Added capability to use existing secret created inside K8s

#### :beetle: Bug Fixes

- Fixed logic for service and statefulset comparison in K8s

### v0.4.0
##### February 6, 2021

#### :tada: Features

- Add Nodeport support for Kubernetes service

#### :beetle: Bug Fixes

- Updated helm chart with latest CRD configuration
- Optimized helm chart
- RBAC issus fixed

### v0.3.0
##### Decemeber 30, 2020

#### :tada: Features

- Upgraded operator-sdk version to v1.0.3
- Added capability to watch multiple namespaces
- Added CI workflow pipeline for golang

#### :beetle: Bug Fixes

- Password updation bug https://github.com/OT-CONTAINER-KIT/redis-operator/issues/21
- POD recovery, Can't Sync pods IP to nodes.conf https://github.com/OT-CONTAINER-KIT/redis-operator/issues/20
- Directory creation (permission issue) https://github.com/OT-CONTAINER-KIT/redis-operator/issues/19

### v0.2.0
##### July 1, 2020

#### :tada: Features

- Added documentation site for better management
- Added YAML validation for redis resource
- Added resources in redis exporter
- Structured complete YAML manifests
- Added service type for redis service
- Updated helm chart with better practices

#### :beetle: Bug Fixes

- Fixed redis cluster failover bug

### v0.0.1 (Initial Release)
##### February 21, 2020

#### :tada: Features

- Cluster/Standalone redis setup
- Password and password-less setup
- Node selector and affinity
- SecurityContext
- Priority Class
- Monitoring support
- PVC and resources support
