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

{{/*
Convert dependency name to environment variable format:
"uniproxy-02" → "UNIPROXY_02"
*/}}
{{- define "dephealth-uniproxy.envName" -}}
{{- . | upper | replace "-" "_" -}}
{{- end -}}

{{/*
Build DEPHEALTH_DEPS value from connections list.
Format: "name1:type1,name2:type2,..."
*/}}
{{- define "dephealth-uniproxy.depsValue" -}}
{{- $deps := list -}}
{{- range . -}}
  {{- $deps = append $deps (printf "%s:%s" .name .type) -}}
{{- end -}}
{{- join "," $deps -}}
{{- end -}}
