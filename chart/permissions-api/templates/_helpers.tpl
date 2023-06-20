{{/* vim: set filetype=mustache: */}}

{{- define "permapi.listenPort" }}
{{- .Values.config.server.port | default 8080 }}
{{- end }}

{{- define "permapi.server.volumes" }}
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

{{- define "permapi.server.volumeMounts" }}
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

{{- define "permapi.worker.volumes" }}
{{- if or .Values.config.spicedb.caSecretName .Values.config.spicedb.policyConfigMapName .Values.config.events.nats.credsSecretName }}
{{- with .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.events.nats.credsSecretName }}
- name: nats-creds
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

{{- define "permapi.worker.volumeMounts" }}
{{- if or .Values.config.spicedb.caSecretName .Values.config.spicedb.policyConfigMapName .Values.config.events.nats.credsSecretName }}
{{- if .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  mountPath: /etc/ssl/spicedb/
{{- end }}
{{- if .Values.config.events.nats.credsSecretName }}
- name: nats-creds
  mountPath: /nats
{{- end }}
{{- if .Values.config.spicedb.policyConfigMapName }}
- name: policy-file
  mountPath: /policy
{{- end }}
{{- else -}}
[]
{{- end }}
{{- end }}
