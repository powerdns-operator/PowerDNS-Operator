{{/*
Expand the name of the chart.
*/}}
{{- define "powerdns-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "powerdns-operator.fullname" -}}
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
{{- define "powerdns-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "powerdns-operator.labels" -}}
helm.sh/chart: {{ include "powerdns-operator.chart" . }}
{{ include "powerdns-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.commonLabels}}
{{ toYaml .Values.commonLabels }}
{{- end }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "powerdns-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "powerdns-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "powerdns-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "powerdns-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine the image to use
*/}}
{{- define "powerdns-operator.image" -}}
{{ printf "%s:v%s" .image.repository (.image.tag | default .chartAppVersion) }}
{{- end }}

{{/*
Return true if the OpenShift is the detected platform
Usage:
{{- include "powerdns-operator.isOpenShift" . -}}
*/}}
{{- define "powerdns-operator.isOpenShift" -}}
{{- if .Capabilities.APIVersions.Has "security.openshift.io/v1" -}}
{{- true -}}
{{- end -}}
{{- end -}}

{{/*
Render the securityContext based on the provided securityContext
  {{- include "powerdns-operator.renderSecurityContext" (dict "securityContext" .Values.securityContext "context" $) -}}
*/}}
{{- define "powerdns-operator.renderSecurityContext" -}}
{{- $adaptedContext := .securityContext -}}
{{- if .context.Values.global.compatibility -}}
  {{- if .context.Values.global.compatibility.openshift -}}
    {{- if or (eq .context.Values.global.compatibility.openshift.adaptSecurityContext "force") (and (eq .context.Values.global.compatibility.openshift.adaptSecurityContext "auto") (include "powerdns-operator.isOpenShift" .context)) -}}
      {{/* Remove OpenShift managed fields */}}
      {{- $adaptedContext = omit $adaptedContext "fsGroup" "runAsUser" "runAsGroup" -}}
      {{- if not .securityContext.seLinuxOptions -}}
        {{- $adaptedContext = omit $adaptedContext "seLinuxOptions" -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- omit $adaptedContext "enabled" | toYaml -}}
{{- end -}}

{{/*
Create the name for the credentials secret.
*/}}
{{- define "powerdns-operator.credentials.name" -}}
{{- if .Values.credentials.existingSecret -}}
  {{- .Values.credentials.existingSecret -}}
{{- else -}}
  {{ default (include "powerdns-operator.fullname" .) .Values.credentials.name }}
{{- end -}}
{{- end -}}