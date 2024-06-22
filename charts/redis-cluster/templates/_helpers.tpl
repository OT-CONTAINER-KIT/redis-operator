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
{{- range $labelkey, $labelvalue := .Values.labels }}
{{ $labelkey}}: {{ $labelvalue }}
{{- end }}
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
{{- if .securityContext }}
securityContext:
  {{- toYaml .securityContext | nindent 2 }}
{{- end }}
{{- end -}}

{{/* Generate sidecar properties */}}
{{- define "sidecar.properties" -}}
{{- with .Values.sidecars }}
name: {{ .name }}
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
{{- end }}
{{- end -}}

{{/* Generate init container properties */}}
{{- define "initContainer.properties" -}}
{{- with .Values.initContainer }}
{{- if .enabled }}
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

