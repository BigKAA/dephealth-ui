{{/*
Full image path: registry/image:tag
*/}}
{{- define "dephealth-uniproxy.image" -}}
{{- if .Values.global.pushRegistry -}}
{{ .Values.global.pushRegistry }}/{{ .Values.image.name }}:{{ .Values.image.tag }}
{{- else -}}
{{ .Values.image.name }}:{{ .Values.image.tag }}
{{- end -}}
{{- end -}}

{{/*
Common labels for all resources.
*/}}
{{- define "dephealth-uniproxy.labels" -}}
app.kubernetes.io/part-of: dephealth
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}
