.. _installation:

####
Helm
####

The easiest way to install a redis operator is using Helm chart. The operator helm chart is developed on the helm=>3.0.0version.

.. code:: bash

    $ # Deploy the redis-operator
    $ helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/
    $ helm upgrade redis-operator ot-helm/redis-operator --install --namespace redis-operator

After deployment, verify the installation of operator

.. code:: bash

    $ # Testing the redis operator
    $ helm test redis-operator --namespace redis-operator

#######
Kubectl
#######

If you are a beginner with Kubernetes and don't want to go in the complexity of Helm, you can use the beloved tool of Kubernetes client like `kubectl`.

**Create the CRD in Kubernetes cluster**

.. code:: bash

    $ kubectl apply -f deploy/crds/redis.opstreelabs.in_redis_crd.yaml

**Configure RBAC for Operator**

.. code:: bash

    $ kubectl apply -f deploy/role.yaml
    $ kubectl apply -f deploy/service_account.yaml
    $ kubectl apply -f deploy/role_binding.yaml

**Deploy Operator in Cluster**

.. code:: bash

    $ kubectl apply -f deploy/operator.yaml
