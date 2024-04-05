{{/* vim: set filetype=mustache: */}}

{{- define "permapi.listenPort" }}
{{- .Values.config.server.port | default 8080 }}
{{- end }}

{{- define "permapi.server.volumes" }}
- name: app-config
  configMap:
    name: {{ include "common.names.name" . }}-server-config
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
{{- with .Values.config.crdb.caSecretName }}
- name: crdb-ca
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.spicedb.policyConfigMapName }}
- name: policy-files
  configMap:
    name: {{ . }}
{{- end }}
{{- end }}

{{- define "permapi.server.volumeMounts" }}
- name: app-config
  mountPath: /config/
{{- if .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  mountPath: /etc/ssl/spicedb/
{{- end }}
{{- if .Values.config.events.nats.credsSecretName }}
- name: nats-creds
  mountPath: /nats
{{- end }}
{{- if .Values.config.crdb.caSecretName }}
- name: crdb-ca
  mountPath: {{ .Values.config.crdb.caMountPath }}
{{- end }}
{{- if .Values.config.spicedb.policyConfigMapName }}
- name: policy-files
  mountPath: {{ .Values.config.spicedb.policyConfigMapMountPoint }}
{{- end }}
{{- end }}

{{- define "permapi.worker.volumes" }}
- name: app-config
  configMap:
    name: {{ include "common.names.name" . }}-worker-config
{{- with .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.crdb.caSecretName }}
- name: crdb-ca
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.events.nats.credsSecretName }}
- name: nats-creds
  secret:
    secretName: {{ . }}
{{- end }}
{{- with .Values.config.spicedb.policyConfigMapName }}
- name: policy-files
  configMap:
    name: {{ . }}
{{- end }}
{{- end }}

{{- define "permapi.worker.volumeMounts" }}
- name: app-config
  mountPath: /config/
{{- if .Values.config.spicedb.caSecretName }}
- name: spicedb-ca
  mountPath: /etc/ssl/spicedb/
{{- end }}
{{- if .Values.config.crdb.caSecretName }}
- name: crdb-ca
  mountPath: {{ .Values.config.crdb.caMountPath }}
{{- end }}
{{- if .Values.config.events.nats.credsSecretName }}
- name: nats-creds
  mountPath: /nats
{{- end }}
{{- if .Values.config.spicedb.policyConfigMapName }}
- name: policy-files
  mountPath: {{ .Values.config.spicedb.policyConfigMapMountPoint }}
{{- end }}
{{- end }}
