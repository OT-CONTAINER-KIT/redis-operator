{{/* vim: set filetype=mustache: */}}

{{- define "redis-backup.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "redis-backup.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "redis-backup.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "redis-backup.labels" -}}
app.kubernetes.io/name: {{ include "redis-backup.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: backup
helm.sh/chart: {{ include "redis-backup.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{- define "redis-backup.selectorLabels" -}}
app.kubernetes.io/name: {{ include "redis-backup.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: backup
{{- end -}}

{{- define "redis-backup.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "redis-backup.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "redis-backup.secretName" -}}
{{- if .Values.secret.create -}}
{{- default (include "redis-backup.fullname" .) .Values.secret.name -}}
{{- else -}}
{{- .Values.existingSecret -}}
{{- end -}}
{{- end -}}

{{/*
Fail fast if required values are missing when the chart is enabled.
Called from each rendered resource template.
*/}}
{{- define "redis-backup.validate" -}}
{{- if .Values.enabled -}}
{{- if not .Values.clusterName -}}
{{- fail "redis-backup: .Values.clusterName is required when .Values.enabled is true" -}}
{{- end -}}
{{- if and .Values.secret.create .Values.existingSecret -}}
{{- fail "redis-backup: set exactly one of .Values.secret.create or .Values.existingSecret, not both" -}}
{{- end -}}
{{- if and (not .Values.secret.create) (not .Values.existingSecret) -}}
{{- fail "redis-backup: set either .Values.secret.create (with .Values.secret.stringData) or .Values.existingSecret" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Pod spec shared between the Job and CronJob templates.
*/}}
{{- define "redis-backup.podSpec" -}}
serviceAccountName: {{ include "redis-backup.serviceAccountName" . }}
restartPolicy: {{ .Values.job.restartPolicy }}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.podSecurityContext }}
securityContext:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
containers:
  - name: backup
    image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
    imagePullPolicy: {{ .Values.image.pullPolicy }}
    command: ["/bin/bash", "-c", "/backup/backup.bash"]
    {{- with .Values.securityContext }}
    securityContext:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    envFrom:
      - secretRef:
          name: {{ include "redis-backup.secretName" . }}
    env:
      - name: CLUSTER_NAME
        value: {{ .Values.clusterName | quote }}
      - name: CLUSTER_NAMESPACE
        value: {{ default .Release.Namespace .Values.clusterNamespace | quote }}
      {{- with .Values.extraEnv }}
      {{- toYaml . | nindent 6 }}
      {{- end }}
    {{- with .Values.resources }}
    resources:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with .Values.extraVolumeMounts }}
    volumeMounts:
      {{- toYaml . | nindent 6 }}
    {{- end }}
{{- with .Values.extraVolumes }}
volumes:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}
