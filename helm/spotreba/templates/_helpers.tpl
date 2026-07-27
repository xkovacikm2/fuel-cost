{{/*
Expand the name of the chart.
*/}}
{{- define "spotreba.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "spotreba.fullname" -}}
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
{{- define "spotreba.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "spotreba.labels" -}}
helm.sh/chart: {{ include "spotreba.chart" . }}
{{ include "spotreba.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "spotreba.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spotreba.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL host
*/}}
{{- define "spotreba.postgresqlHost" -}}
{{- printf "%s-postgresql" (include "spotreba.fullname" .) }}
{{- end }}

{{/*
Database URL
*/}}
{{- define "spotreba.databaseUrl" -}}
{{- printf "postgresql://%s:$(SPOTREBA_DATABASE_PASSWORD)@%s:5432/%s" .Values.postgresql.auth.username (include "spotreba.postgresqlHost" .) .Values.postgresql.auth.database }}
{{- end }}
