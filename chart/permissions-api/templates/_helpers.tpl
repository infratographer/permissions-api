{{/* vim: set filetype=mustache: */}}

{{- define "permapi.listenPort" }}
{{- .Values.config.server.port | default 8080 }}
{{- end }}
