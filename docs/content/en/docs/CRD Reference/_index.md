---
title: "CRD Reference"
linkTitle: "CRD Reference"
weight: 6
date: 2022-11-02T00:19:19Z
description: >
  Reference documentation for CRD spec of Redis and Redis cluster
---

A resource in the Kubernetes API is an endpoint that persists a collection of a particular type of object. For example, several built-in objects, like pods and deployments, are exposed via an endpoint, and the API server manages their lifecycle. Kubernetes provides you with an option of extending your object using CRD so that you can introduce your API to the Kubernetes cluster per your requirement. Using CRD on Kubernetes, you are free to define, create, and persist any custom object.

<div align="center">
    <img src="../../../images/crd-architecture.png">
</div>
