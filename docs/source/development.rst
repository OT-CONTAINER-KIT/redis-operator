.. _development:

Pre-requisites
==============

**Access to Kubernetes cluster**

First of all, you will need access to a Kubernetes cluster. The easiest way to start is minikube.

- `Virtualbox <https://www.virtualbox.org/wiki/Downloads>`__ - hypervisor to run a Kubernetes cluster
- `Minikube <https://kubernetes.io/docs/setup/minikube/>`__ - for Kubernetes cluster creation on local machine
- `Kubectl <https://kubernetes.io/docs/tasks/tools/install-kubectl/>`__ - to interact with Kubernetes cluster


**Tools to build an Operator**

Apart from kubernetes cluster, there are some tools which are needed to build and test the redis operator.

- `Git <https://git-scm.com/downloads>`
- `Go <https://golang.org/dl/>`
- `Docker <https://docs.docker.com/install/>`
- `Operator SDK <https://github.com/operator-framework/operator-sdk/blob/v0.8.1/doc/user/install-operator-sdk.md>`
- `Make <https://www.gnu.org/software/make/manual/make.html>`

Build Locally
=============

To achieve this, execute this command:-

.. code:: bash

    $ make build

Build Image
===========

Redis operator gets packaged as a container image for running on the Kubernetes cluster. These instructions will guide you to build an image.

.. code:: bash

    $ make build-image

Testing
=======

If you want to play it on Kubernetes. You can use a minikube.

.. code:: bash

    # Start minikube
    $ minikube start --vm-driver virtualbox

    # Deploy the image on minikube
    $ helm upgrade redis-cluster ot-helm/redis-setup \
        --set redisSetup.setupMode="cluster" \
        --set redisSetup.clusterSize=3 \
        --install --namespace redis-operator

Run Tests
=========

.. code:: bash

    $ make test

