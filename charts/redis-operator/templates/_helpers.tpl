{{/* vim: set filetype=mustache: */}}

{{/* Define issuer spec based on the type */}}
{{- define "redis-operator.issuerSpec" -}}
{{- if eq .Values.issuer.type "acme" }}
acme:
  email: {{ .Values.issuer.email }}
  server: {{ .Values.issuer.server }}
  privateKeySecretRef:
    name: {{ .Values.issuer.privateKeySecretName }}
  solvers:
  - http01:
      ingress:
        class: {{ .Values.issuer.solver.ingressClass }}
{{- else }}
selfSigned: {}
{{- end }}
{{- end -}}

{{/* Common labels */}}
{{- define "redisOperator.labels" -}}
app.kubernetes.io/name: {{ .Values.redisOperator.name }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/component: operator
app.kubernetes.io/part-of: {{ .Release.Name }}
{{- end }}

{{/* Selector labels */}}
{{- define "redisOperator.selectorLabels" -}}
name: {{ .Values.redisOperator.name }}
{{- end }}