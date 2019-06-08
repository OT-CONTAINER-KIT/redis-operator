# Redis Operator

This is Redis Operator which will create/manage Redis on the top of the Kubernetes. The project is inspired by the **[Operator Framework](https://coreos.com/operators/)** which is initiated by the **[CoreOS](https://coreos.com/)**.

This project is maintained by **[OpsTree Solutions](https://www.opstree.com)**
## Requirements
- **Golang** - If you want to do development
- **Kubernetes 1.9+** - This operator supports Kubernetes 1.9+ versions

## Overview

Redis Operator deploy and manage the Redis instances in form of **cluster** or **Master and Slave** depending upon on your configuration

Things you should know about Redis Operator:-
- 3 is a minimum number of Redis instances.
- Redis 5.0 is the minimum supported version.
- Redis Operator is not a distributed system. It leverages a simple leader election protocol. You can run multiple instances of Redis Operator.

