{{- define "gvList" -}}
{{- $groupVersions := . -}}
---
title: "API Reference Documentation"
linkTitle: "API Docs"
weight: 10
date: 2025-01-27
description: >
  Complete API reference documentation for Redis Operator CRDs
---

# API Reference

## Packages
{{- range $groupVersions }}
- {{ markdownRenderGVLink . }}
{{- end }}

{{ range $groupVersions }}
{{ template "gvDetails" . }}
{{ end }}

{{- end -}}
