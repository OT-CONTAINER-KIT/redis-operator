### v0.15.0 
##### July 17, 2023 Latest

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
