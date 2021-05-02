### v0.5.0
**May 1, 2021**

**:tada: Features**

- Added support for recovering redis nodes from failover
- Added toleration support for redis statefuls
- Added capability to use existing secret created inside K8s

**:beetle: Bug Fixes**

- Fixed logic for service and statefulset comparison in K8s

### v0.4.0
**February 6, 2021**

**:tada: Features**

- Add Nodeport support for Kubernetes service

**:beetle: Bug Fixes**

- Updated helm chart with latest CRD configuration
- Optimized helm chart
- RBAC issus fixed

### v0.3.0
**Decemeber 30, 2020**

**:tada: Features**

- Upgraded operator-sdk version to v1.0.3
- Added capability to watch multiple namespaces
- Added CI workflow pipeline for golang

**:beetle: Bug Fixes**

- Password updation bug https://github.com/OT-CONTAINER-KIT/redis-operator/issues/21
- POD recovery, Can't Sync pods IP to nodes.conf https://github.com/OT-CONTAINER-KIT/redis-operator/issues/20
- Directory creation (permission issue) https://github.com/OT-CONTAINER-KIT/redis-operator/issues/19

### v0.2.0
**July 1, 2020**

**:tada: Features**

- Added documentation site for better management
- Added YAML validation for redis resource
- Added resources in redis exporter
- Structured complete YAML manifests
- Added service type for redis service
- Updated helm chart with better practices

**:beetle: Bug Fixes**

- Fixed redis cluster failover bug

### v0.1.0
**February 21, 2020**

**:tada: Features**

- Cluster/Standalone redis setup
- Password and password-less setup
- Node selector and affinity
- SecurityContext
- Priority Class
- Monitoring support
- PVC and resources support
