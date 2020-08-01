# Helm Configurations

These are helm configurations which can be modified as per the requirement.

|**Value**|**Available Option**|**Default Value**|**Description**|
|---------|--------------------|-----------------|---------------|
| name | *Any string* | redis | Name of the redis setup which will be done by operator |
| setupMode | <ul><li>cluster</li><li>standalone</li></ul> | - | Setup mode for redis |
| cluster.size | *Any integer* | 3 | Size of the redis node for cluster mode setup |
| cluster.master.serviceType | <ul><li>ClusterIP</li><li>LoadBalancer</li><li>NodePort</li></ul> | ClusterIP | Service type for redis master nodes |
| cluster.slave.serviceType | <ul><li>ClusterIP</li><li>LoadBalancer</li><li>NodePort</li></ul> | ClusterIP | Service type for redis slave nodes |
| global.image | *Valid Opstree Image* | quay.io/opstree/redis | Name of the image for setting up redis |
| global.tag | *Valid Tag* | v0.2 | Image version for redis setup |
| global.imagePullPolicy | <ul><li>IfNotPresent</li><li>Always</li></ul> | IfNotPresent | Image pull policy for redis statefulsets and pods |
| global.password | *Any strong Password* | Opstree@1234 | Password for redis setup, make it blank or comment if you don't want password |
| global.resources | *K8s Resources* | - | Requests and limits for redis pods |
| exporter.enabled | <ul><li>true</li><li>false</li></ul> | true | Redis exporter is enabled or not |
| exporter.image | *Valid Opstree Image* | quay.io/opstree/redis-exporter | Name of the image for setting up redis exporter |
| exporter.tag | *Valid Tag* | 0.2 | Image version for redis exporter |
| exporter.imagePullPolicy | <ul><li>IfNotPresent</li><li>Always</li></ul> | IfNotPresent | Image pull policy for redis exporter sidecar |
| priorityClassName | *Valid Priority Class* | - | Name of the kubernetes priorityclass which you want to associate with redis setup |
| nodeSelector | *Valid Node Selector* | - | Map of the labels which you want to use as as nodeSelector |
| storageSpec | *Valid Storage Spec* | - | Kubernetes storage definition for redis pods |
| securityContext | *K8s SecurityContext* | - | Kubernetes security context for redis pods |
| affinity | *K8s Affinity* | - | Node and pod affinity for redis pods |
