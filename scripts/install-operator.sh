#!/bin/bash

kubectl apply -f CODES/golang/redis-operator-1/config/crd/bases/
kubectl apply -f CODES/golang/redis-operator-1/config/manager/manager.yaml
kubectl apply -f CODES/golang/redis-operator-1/config/rbac/serviceaccount.yaml
kubectl apply -f CODES/golang/redis-operator-1/config/rbac/role.yaml
kubectl apply -f CODES/golang/redis-operator-1/config/rbac/role_binding.yaml
