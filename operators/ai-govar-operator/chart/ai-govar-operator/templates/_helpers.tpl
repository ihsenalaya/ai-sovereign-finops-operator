{{- define "operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "operator.govArCalibrationProducerServiceAccountName" -}}
{{- if .Values.govArCalibrationProducer.serviceAccount.create -}}
{{- default (printf "%s-gov-ar-calibration-producer" (include "operator.fullname" .)) .Values.govArCalibrationProducer.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- required "govArCalibrationProducer.serviceAccount.name is required when serviceAccount.create=false" .Values.govArCalibrationProducer.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "operator.govArCalibrationProducerImage" -}}
{{- if .Values.govArCalibrationProducer.image.digest -}}
{{- printf "%s@%s" .Values.govArCalibrationProducer.image.repository .Values.govArCalibrationProducer.image.digest -}}
{{- else -}}
{{- $tag := .Values.govArCalibrationProducer.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.govArCalibrationProducer.image.repository $tag -}}
{{- end -}}
{{- end -}}

{{- define "operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "operator.labels" -}}
app.kubernetes.io/name: {{ include "operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ai-sovereign-finops-operator
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "operator.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "operator.govArAdmissionServiceAccountName" -}}
{{- if .Values.govArAdmission.serviceAccount.create -}}
{{- default (printf "%s-gov-ar-admission" (include "operator.fullname" .)) .Values.govArAdmission.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- required "govArAdmission.serviceAccount.name is required when serviceAccount.create=false" .Values.govArAdmission.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "operator.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{- define "operator.govArAdmissionImage" -}}
{{- if .Values.govArAdmission.image.digest -}}
{{- printf "%s@%s" .Values.govArAdmission.image.repository .Values.govArAdmission.image.digest -}}
{{- else -}}
{{- $tag := .Values.govArAdmission.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.govArAdmission.image.repository $tag -}}
{{- end -}}
{{- end -}}
