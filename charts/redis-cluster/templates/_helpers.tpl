{{/* vim: set filetype=mustache: */}}

{{/* Define common labels */}}
{{- define "common.labels" -}}
app.kubernetes.io/name: {{ .Values.redisCluster.name | default .Release.Name }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Values.redisCluster.name | default .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/component: middleware
{{- if .Values.labels }}
{{ toYaml .Values.labels }}
{{- end }}
{{- end -}}

{{/* Define common annotations */}}
{{- define "common.annotations" -}}
{{- if .Values.annotations }}
{{ toYaml .Values.annotations }}
{{- end }}
{{- end -}}

{{/* Helper for Redis Cluster (leader & follower) */}}
{{- define "redis.role" -}}
{{- if .affinity }}
affinity:
  {{- toYaml .affinity | nindent 2 }}
{{- end }}
{{- if .tolerations }}
tolerations:
  {{- toYaml .tolerations | nindent 2 }}
{{- end }}
{{- if .pdb.enabled  }}
pdb:
  enabled: {{ .pdb.enabled }}
  maxUnavailable: {{ .pdb.maxUnavailable }}
  minAvailable: {{ .pdb.minAvailable }}
{{- end }}
{{- if .nodeSelector }}
nodeSelector:
  {{- toYaml .nodeSelector | nindent 2 }}
{{- end }}
{{- if .topologySpreadConstraints }}
topologySpreadConstraints:
  {{- toYaml .topologySpreadConstraints | nindent 2 }}
{{- end }}
{{- if .securityContext }}
securityContext:
  {{- toYaml .securityContext | nindent 2 }}
{{- end }}
{{- if .livenessProbe }}
livenessProbe:
  {{- toYaml .livenessProbe | nindent 2 }}
{{- end }}
{{- if .readinessProbe }}
readinessProbe:
  {{- toYaml .readinessProbe | nindent 2 }}
{{- end }}
{{- end -}}


{{/* Generate init container properties */}}
{{- define "initContainer.properties" -}}
{{- with .Values.initContainer }}
{{- if .enabled }}
enabled: {{ .enabled }}
image: {{ .image }}
{{- if .imagePullPolicy }}
imagePullPolicy: {{ .imagePullPolicy }}
{{- end }}
{{- if .resources }}
resources:
  {{ toYaml .resources | nindent 2 }}
{{- end }}
{{- if .env }}
env:
{{ toYaml .env | nindent 2 }}
{{- end }}
{{- if .command }}
command:
{{ toYaml .command | nindent 2 }}
{{- end }}
{{- if .args }}
args:
{{ toYaml .args | nindent 2 }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

{{/* Resolve leader/follower replica count, falling back to clusterSize only when replicas is unset (nil), so an explicit 0 is preserved */}}
{{/* Usage: include "redis.replicas" (dict "replicas" .Values.redisCluster.leader.replicas "clusterSize" .Values.redisCluster.clusterSize) */}}
{{- define "redis.replicas" -}}
{{- if kindIs "invalid" .replicas -}}
{{- .clusterSize -}}
{{- else -}}
{{- .replicas -}}
{{- end -}}
{{- end -}}

{{/* Validate service type and return the value */}}
{{/* Usage: include "common.validateServiceType" (dict "serviceType" .Values.xxx.serviceType "name" (.Values.xxx.name | default .Release.Name)) */}}
{{- define "common.validateServiceType" -}}
{{- $allowedServiceTypes := list "ClusterIP" "NodePort" "LoadBalancer" -}}
{{- $serviceType := .serviceType | default "ClusterIP" -}}
{{- if has $serviceType $allowedServiceTypes -}}
{{- $serviceType -}}
{{- else -}}
{{- fail (printf "%s serviceType must be one of ClusterIP, NodePort, LoadBalancer; got: %s" .name $serviceType) -}}
{{- end -}}
{{- end -}}
