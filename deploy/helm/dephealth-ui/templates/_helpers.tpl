{{/*
Full image path: registry/image:tag
*/}}
{{- define "dephealth-ui.image" -}}
{{- if .Values.global.pushRegistry -}}
{{ .Values.global.pushRegistry }}/{{ .Values.image.name }}:{{ .Values.image.tag }}
{{- else -}}
{{ .Values.image.name }}:{{ .Values.image.tag }}
{{- end -}}
{{- end -}}

{{/*
Common labels for all resources.
*/}}
{{- define "dephealth-ui.labels" -}}
app.kubernetes.io/name: dephealth-ui
app.kubernetes.io/part-of: dephealth
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "dephealth-ui.selectorLabels" -}}
app: dephealth-ui
app.kubernetes.io/name: dephealth-ui
app.kubernetes.io/part-of: dephealth
{{- end -}}

{{/*
Target namespace.
*/}}
{{- define "dephealth-ui.namespace" -}}
{{ .Values.global.namespace | default "dephealth-ui" }}
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "dephealth-ui.serviceAccountName" -}}
{{- if .Values.serviceAccount.name -}}
{{ .Values.serviceAccount.name }}
{{- else -}}
dephealth-ui
{{- end -}}
{{- end -}}
