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

{{/*
Validate webhook and cert-manager configuration
*/}}
{{- define "redis-operator.validateConfig" -}}
{{- if and (not .Values.redisOperator.webhook) .Values.certmanager.enabled -}}
{{- fail "certmanager.enabled should not be true when webhook is disabled" -}}
{{- end -}}
{{- end -}}

{{/*
Validate and normalize the RBAC scope.
Returns "cluster" (the default when unset) or "namespace".
*/}}
{{- define "redis-operator.rbacScope" -}}
{{- $scope := .Values.rbac.scope | default "cluster" -}}
{{- if and (ne $scope "cluster") (ne $scope "namespace") -}}
{{- fail (printf "rbac.scope must be either \"cluster\" or \"namespace\", got %q" $scope) -}}
{{- end -}}
{{- $scope -}}
{{- end -}}