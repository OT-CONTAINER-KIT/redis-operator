Quickstart
##########

.. toctree::
   :maxdepth: 2

In this document you will find a step-by-step guide on how to get redis-operator running in a local Minikube cluster. You will run a simple standalone and cluster mode of Redis.

Pre-requisites
**************

In order to setup redis-operator, you'll need access to a Kubernetes cluster:-

- [Virtualbox](https://www.virtualbox.org/wiki/Downloads) - hypervisor to run a Kubernetes cluster
- [Minikube](https://kubernetes.io/docs/setup/minikube/) - for Kubernetes cluster creation on local machine
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) - to interact with Kubernetes cluster

Deploying Redis Operator(Minikube)
**********************************

**Start a local minikube cluster**

Minikube is a minimal Kubernetes cluster run in a virtual machine (here in VirtualBox).

.. code-block:: bash
    $ minikube start --vm-driver virtualbox


From now on your local Kubernetes client kubectl is configured to use your just started Minikube cluster.

**Create a new namespace**

First, we need to create a namespace for our resources to be deployed in. This is for the sake of separation and keeping order:

.. code-block:: bash
    $ kubectl create namespace redis-operator


Redis operator by default watches for every change in Redis Configuration.

Standalone Setup
****************

.. code-block:: bash
    $ kubectl apply -f example/redis-standalone-example.yaml -n redis-operator


Cluster Setup
*************

.. code-block:: bash
    $ kubectl apply -f example/redis-cluster-example.yaml -n redis-operator
