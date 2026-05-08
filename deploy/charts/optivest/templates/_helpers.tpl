{{/*
Expand the name of the chart.
*/}}
{{- define "optivest.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "optivest.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "optivest.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "optivest.labels" -}}
helm.sh/chart: {{ include "optivest.chart" . }}
{{ include "optivest.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "optivest.selectorLabels" -}}
app.kubernetes.io/name: {{ include "optivest.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "optivest.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "optivest.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Migrate Job name. Append "-migrate" to the standard fullname so
operators can spot the Job among the release's other resources at a
glance, while still respecting the 63-char DNS label limit.
*/}}
{{- define "optivest.migrate.fullname" -}}
{{- printf "%s-migrate" (include "optivest.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Migrate image reference. tag falls back to .Chart.AppVersion so a
chart consumer can pin BOTH the API image AND the migrate image by
bumping appVersion alone — the schema baked into the migrate image
moves in lockstep with the API binary that expects it.
*/}}
{{- define "optivest.migrate.image" -}}
{{- $tag := default .Chart.AppVersion .Values.migrate.image.tag }}
{{- printf "%s:%s" .Values.migrate.image.repository $tag }}
{{- end }}

{{/*
Migrate Job labels. Mirrors the standard optivest.labels shape but
sets app.kubernetes.io/component=migrate so dashboards, log
collectors, and `kubectl get -l` queries can pick the Job out of
the release's other resources.

Job selectors are intentionally NOT exposed here — Kubernetes
auto-generates controller-uid + job-name labels and uses those for
the Pod selector, which avoids the API Deployment and the Job
matching each other if their labels overlap.
*/}}
{{- define "optivest.migrate.labels" -}}
helm.sh/chart: {{ include "optivest.chart" . }}
app.kubernetes.io/name: {{ include "optivest.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: migrate
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
