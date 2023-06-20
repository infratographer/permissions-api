{{/* vim: set filetype=mustache: */}}

{{- define "permapi.listenPort" }}
{{- .Values.config.server.port | default 8080 }}
{{- end }}

{{- define "permapi.volumes" }}
{{- if or .Values.config.spicedb.caSecretName .Values.config.spicedb.policyConfigMapName }}
{{- with .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.spicedb.policyConfigMapName }}
- name: policy-file
  configMap:
    name: {{ . }}
{{- end }}
{{- else -}}
[]
{{- end }}
{{- end }}

{{- define "permapi.volumeMounts" }}
{{- if or .Values.config.spicedb.caSecretName .Values.config.spicedb.policyConfigMapName }}
{{- if .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  mountPath: /etc/ssl/spicedb/
{{- end }}
{{- if .Values.config.spicedb.policyConfigMapName }}
- name: policy-file
  mountPath: /policy
{{- end }}
{{- else -}}
[]
{{- end }}
{{- end }}
