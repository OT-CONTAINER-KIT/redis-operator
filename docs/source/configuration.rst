.. _configuration:

Mode
====

Mode of the redis setup. Available Options:-

- cluster - For cluster mode setup of redis
- standalone - For standalone setup of redis

.. code:: yaml

    mode: cluster

Size
====

Size of the redis cluster pods. Available Options:-

- A valid integer

.. code:: yaml

    size: 3

Global
======

In the global section, we define similar configurations across the redis nodes.

.. code:: yaml

    global:
      image: opstree/redis:v2.0
      imagePullPolicy: IfNotPresent
      password: "Opstree@1234"
      resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 100m
        memory: 128Mi

Master
======

Configuration specific to master nodes of Redis

.. code:: yaml

    master:
      service:
        type: ClusterIP

Slave
=====

Configuration specific to slave nodes of Redis

.. code:: yaml

    slave:
      service:
        type: ClusterIP

Redis Exporter
==============

Redis Exporter Configurations.

.. code:: yaml

    redisExporter:
      enabled: true
      image: quay.io/opstree/redis-exporter:1.0
      imagePullPolicy: Always
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 100m
          memory: 128Mi

Storage
=======

Storage definition for redis nodes

.. code:: yaml

    storage:
      volumeClaimTemplate:
        spec:
          storageClassName: csi-cephfs-sc
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 1Gi
        selector: {}

Priority Class
==============

Name of the Kubernetes priority class which you want to associate with redis setup

.. code:: yaml

    priorityClassName: priority-100

Node Selector
=============

Map of the labels which you want to use as nodeSelector

.. code:: yaml

    nodeSelector:
      memory: medium

Security Context
================

Kubernetes security context for redis pods

.. code:: yaml

    securityContext:
      runAsUser: 1000

Affinity
========

Node and pod affinity for redis pods

.. code:: yaml

    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: disktype
              operator: In
              values:
              - ssd
