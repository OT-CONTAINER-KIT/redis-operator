#!/bin/bash

kubectl apply -f config/crd/bases/
kubectl apply -f config/manager/manager.yaml
kubectl apply -f config/rbac/serviceaccount.yaml
kubectl apply -f config/rbac/role.yaml
kubectl apply -f config/rbac/role_binding.yaml
