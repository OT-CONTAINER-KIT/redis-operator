---
title: "Release History"
linkTitle: "Release History"
weight: 10
date: 2022-11-02T00:19:19Z
description: >
  Release versions and their description about Redis Operator
---

### v0.15.0 
##### July 17, 2023 Latest

**🐞 Bug Fixes**

- Fix Linter Issue #479 
- Fix exporter ports enabled even when exporters disabled [#484](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/484)
- Corrected scenario "go-get-tool" in makefile [#499](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/499)
- Operator Crash when persistence is false/Disabled [#519](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/519) -- Breaking Change
- call of func checkAttachedSlave for 1 Master Replication [#523](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/523)
- Only created /node-conf VolumeMount for clusters [#532](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/532)
- Redis Sentinel Exporter ports in Env Vars [#533](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/533)
- Init Container tried to mount invalid volume name [#538](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/538)
- Cluster leader failover loop if there is only a single leader [#542](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/542)

**🎉 Features**

- Add RedisExporter for sentinel [#440](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/440)
- Add InitContainer Field [#458](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/458)
- ACL redis via secret [#486](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/486)
- Adding Custom TerminationGracePeriodSeconds and additional fields for Sidecar [#487](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/487)
- Enable Support for Backup and Restore via script [#489](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/489)
- Support Scaling for Redis Cluster [#531](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/531) -- Breaking Change

**🎉 Refactors**

- Fixed StatefulSet(sentinel) Label for Service(Selector) [#442](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/442)
- Declare Module Correctly On sentinel [#478](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/478)
- Manage (Pod and Container) security Context explicitly [#518](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/518)
- Add watchnamespace function as per operator hub [#520](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/520)
- Remove sentinel default validation not effect [#535](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/535)
- Remove sentinel cluster size validation no effect [#536](https://github.com/OT-CONTAINER-KIT/redis-operator/pull/536)

### v0.14.0 
##### Feburary 13, 2023

**🐞 Bug Fixes**

- Added check for persistent volume nil condition
- Fix crash with go panic
- Fix memory address bug and nil pointer
- CR annotations fixes w.r.t. to stateful set
- Fix issues with ARM64 support

**🎉 Features**

- Added serviceType functionality for redis standalone and cluster
- Added feature for additional volume mounts
- Added nodeSelectory and tolerations for redis cluster
- Added recreation logic for redis stateful sets
- Added replication mode support for the redis cluster
- Added sentinel support for replication failover

### v0.13.0
##### November 10, 2022

**🐞 Bug Fixes**

- Fixed multiple follower logic for redis cluster

**🎉 Features**

- Updated all examples for Redis v7
- Revamped documentation with the latest information
- Added pause option for reconcilations
- Added support for arm64
- Added update strategy for statefulset
- Added logic for updating follower replicas
- Added TLS feature for standalone

### v0.12.0
##### October 12, 2022

**🐞 Bug Fixes**

- PDB (Pod disruption budget) creation issue
- Fixed cluster recovery logic
- Fixed IP check and conversion logic
- Persistence issue fix

**🎉 Features**

- Added pvc, pv clusterrole fix
- Support for defining serviceAccount
- Closing of redis client connection
- Added finalizer for statefulset
- Added Prometheus service annotation
- Added support for Redis 7 with DNS hostname

### v0.11.0
**July 5, 2022**

**🐞 Bug Fixes**

- Fix Redis cluster and Redis CRD
- Fixed TLS authentication between redis cluster
- Fixed RBAC policy for PDB
- Redis exporter exception handled
- External service fix

### v0.10.0
**January 26, 2022**

**🎉 Features**

- Added custom probes capability
- Added sidecar support for redis
- Added option for namespaced operator
- Added finalizers for Kubernetes resources
- Adding PodDisruptionBudget support
- Added TLS cluster support
- Pass through Annotations and Labels to all Child resources
- Adding Rudimentry IPv6 Support

**🐞 Bug Fixes**

- Fix up RedisClusterStatus Spec being incorrect object
- Fixed invalid RBAC kustomization
- Fixed RBAC role for operator
- Fixed service creation for leader and follower

### v0.9.0
**November 13, 2021**

**🎉 Features**

- Added RBAC policies for redis operator with least privileges

**🐞 Bug Fixes**

- Fix and updated documentation dependencies
- Test pointers before dereferencing
- Fix panic error of golang for redis exporter
- Fix resource block nil exception for redis exporter

### v0.8.0
**September 3, 2021**

**🎉 Features**

- Added external configuration capability for follower and leader
- Streamlined examples folder with different examples for standalone and cluster
- Added the capability for affinity for leader and follower

**🐞 Bug Fixes**

- Fix bug for non-defined storage
- Fixed secret nil exception bug
- Fixed bug for making redis exporter optional

### v0.7.0
**August 12, 2021**

**🎉 Features**

- Remove all the vulnerable dependencies from docs(NodeJS)
- Added a new grafana dashboard for better monitoring visualization
- Added environment variable support for redis exporter
- Added Image Pull Secret support for private registeries

**🐞 Bug Fixes**

- Fix bug for non-defined storage
- Fixed secret nil exception bug
- Fixed bug for making redis exporter optional

### v0.6.0
**June 12, 2021**

**🎉 Features**

- Breaked the CRDs into Redis standalone cluster setup
- Optimized code configuration for creating Redis cluster
- Removed string secret type and secret type password is only supported
- Structured and optimized golang based codebase
- Removed divisive terminlogies

**🐞 Bug Fixes**

- Fixed logic for service and statefulset comparison in K8s
- Removed the monitor label to resolve service endpoint issue

### v0.5.0
**May 1, 2021**

**🎉 Features**

- Added support for recovering redis nodes from failover
- Added toleration support for redis statefuls
- Added capability to use existing secret created inside K8s

**🐞Bug Fixes**

- Fixed logic for service and statefulset comparison in K8s

### v0.4.0
**February 6, 2021**

**🎉 Features**

- Add Nodeport support for Kubernetes service

**🐞 Bug Fixes**

- Updated helm chart with latest CRD configuration
- Optimized helm chart
- RBAC issus fixed

### v0.3.0
**Decemeber 30, 2020**

**🎉 Features**

- Upgraded operator-sdk version to v1.0.3
- Added capability to watch multiple namespaces
- Added CI workflow pipeline for golang

**🐞 Bug Fixes**

- Password updation bug https://github.com/OT-CONTAINER-KIT/redis-operator/issues/21
- POD recovery, Can't Sync pods IP to nodes.conf https://github.com/OT-CONTAINER-KIT/redis-operator/issues/20
- Directory creation (permission issue) https://github.com/OT-CONTAINER-KIT/redis-operator/issues/19

### v0.2.0
**July 1, 2020**

**🎉 Features**

- Added documentation site for better management
- Added YAML validation for redis resource
- Added resources in redis exporter
- Structured complete YAML manifests
- Added service type for redis service
- Updated helm chart with better practices

**🐞 Bug Fixes**

- Fixed redis cluster failover bug

### v0.1.0
**February 21, 2020**

**🎉 Features**

- Cluster/Standalone redis setup
- Password and password-less setup
- Node selector and affinity
- SecurityContext
- Priority Class
- Monitoring support
- PVC and resources support
