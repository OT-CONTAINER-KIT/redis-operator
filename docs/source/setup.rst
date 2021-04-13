.. _setup:

##########
Standalone
##########

We can use helm and kubectl for deploying the redis standalone server.

Helm
====

.. code:: bash

    # Create redis standalone setup
    $ helm upgrade redis ot-helm/redis-setup \
    --set setupMode="standalone" \
    --install --namespace redis-operator

Kubectl
=======

.. code:: bash

    # Standalone redis deployment
    $ kubectl apply -f example/redis-standalone-example.yaml

#######
Cluster
#######

For setting up the redis cluster we can use helm and kubectl. The only thing which needs to be taken care of is that the minimum count for cluster is 3.

Helm
====

.. code:: bash

    # Create redis cluster setup
    $ helm upgrade redis-cluster ot-helm/redis-setup \
    --set setupMode="cluster" --set cluster.size=3 \
    --install --namespace redis-operator

Kubectl
=======

.. code:: bash

    # Standalone redis deployment
    $ kubectl apply -f example/redis-cluster-example.yaml
