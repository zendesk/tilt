package tiltfile

const kustomizeFileText = `# Example configuration for the webserver
# at https://github.com/monopole/hello
commonLabels:
  app: my-hello

resources:
- deployment.yaml
- service.yaml
- configMap.yaml`

const kustomizeConfigMapText = `apiVersion: v1
kind: ConfigMap
metadata:
  name: the-map
data:
  altGreeting: "Good Morning!"
  enableRisky: "false"`

const kustomizeDeploymentText = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: the-deployment
spec:
  replicas: 3
  template:
    metadata:
      labels:
        deployment: hello
    spec:
      containers:
      - name: the-container
        image: gcr.io/foo
        command: ["/hello",
                  "--port=8080",
                  "--enableRiskyFeature=$(ENABLE_RISKY)"]
        ports:
        - containerPort: 8080
        env:
        - name: ALT_GREETING
          valueFrom:
            configMapKeyRef:
              name: the-map
              key: altGreeting
        - name: ENABLE_RISKY
          valueFrom:
            configMapKeyRef:
              name: the-map
              key: enableRisky`

const kustomizeServiceText = `kind: Service
apiVersion: v1
metadata:
  name: the-service
spec:
  selector:
    deployment: hello
  type: LoadBalancer
  ports:
  - protocol: TCP
    port: 8666
    targetPort: 8080`

const chartYAML = `apiVersion: v1
description: A Helm chart for Kubernetes
name: helloworld-chart
version: 0.1.0`

const chartYAML3 = `apiVersion: v2
name: helloworld-chart
description: A Helm chart for Kubernetes

type: application

version: 0.1.0

appVersion: 1.16.0`

const valuesYAML = `# Default values for helloworld-chart.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.
replicaCount: 1
image:
  repository: nginx
  tag: stable
  pullPolicy: IfNotPresent
service:
  name: nginx
  type: ClusterIP
  externalPort: 80
  internalPort: 80
ingress:
  enabled: false
  # Used to create an Ingress record.
  hosts:
    - chart-example.local
  annotations:
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  tls:
    # Secrets must be manually created in the namespace.
    # - secretName: chart-example-tls
    #   hosts:
    #     - chart-example.local
namespace:
  enabled: false
resources: {}
  # We usually recommend not to specify default resources and to leave this as a conscious
  # choice for the user. This also increases chances charts run on environments with little
  # resources, such as Minikube. If you do want to specify resources, uncomment the following
  # lines, adjust them as necessary, and remove the curly braces after 'resources:'.
  # limits:
  #  cpu: 100m
  #  memory: 128Mi
  # requests:
  #  cpu: 100m
	#  memory: 128Mi`

const valuesYAML3 = `# Default values for helloworld-chart.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

replicaCount: 1

image:
  repository: nginx
  pullPolicy: IfNotPresent

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  # Specifies whether a service account should be created
  create: true
  # The name of the service account to use.
  # If not set and create is true, a name is generated using the fullname template
  name:

podSecurityContext: {}
  # fsGroup: 2000

securityContext: {}
  # capabilities:
  #   drop:
  #   - ALL
  # readOnlyRootFilesystem: true
  # runAsNonRoot: true
  # runAsUser: 1000

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: false
  annotations: {}
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  hosts:
    - host: chart-example.local
      paths: []
  tls: []
  #  - secretName: chart-example-tls
  #    hosts:
  #      - chart-example.local

resources: {}
  # We usually recommend not to specify default resources and to leave this as a conscious
  # choice for the user. This also increases chances charts run on environments with little
  # resources, such as Minikube. If you do want to specify resources, uncomment the following
  # lines, adjust them as necessary, and remove the curly braces after 'resources:'.
  # limits:
  #   cpu: 100m
  #   memory: 128Mi
  # requests:
  #   cpu: 100m
  #   memory: 128Mi

nodeSelector: {}

tolerations: []

affinity: {}`

const valuesDevYAML = `# Dev values for helloworld chart
service:
  name: nginx-dev
`

const helpersTPL = `{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "helloworld-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "helloworld-chart.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
`

const helpersTPL3 = `{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "helloworld-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "helloworld-chart.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "helloworld-chart.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "helloworld-chart.labels" -}}
helm.sh/chart: {{ include "helloworld-chart.chart" . }}
{{ include "helloworld-chart.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "helloworld-chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "helloworld-chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "helloworld-chart.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "helloworld-chart.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}`

const deploymentYAML = `{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "helloworld-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "helloworld-chart.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
`

const deploymentYAML3 = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "helloworld-chart.fullname" . }}
  labels:
    {{- include "helloworld-chart.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "helloworld-chart.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "helloworld-chart.selectorLabels" . | nindent 8 }}
    spec:
    {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
    {{- end }}
      serviceAccountName: {{ include "helloworld-chart.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: "{{ .Values.image.repository }}:{{ .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 80
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /
              port: http
          readinessProbe:
            httpGet:
              path: /
              port: http
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
    {{- end }}
    {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
    {{- end }}`

const ingressYAML = `{{- if .Values.ingress.enabled -}}
{{- $serviceName := include "helloworld-chart.fullname" . -}}
{{- $servicePort := .Values.service.externalPort -}}
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: {{ template "helloworld-chart.fullname" . }}
  labels:
    app: {{ template "helloworld-chart.name" . }}
    chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
    release: {{ .Release.Name }}
    heritage: {{ .Release.Service }}
    namespaceLabel: {{ .Release.Namespace }}
  annotations:
    {{- range $key, $value := .Values.ingress.annotations }}
      {{ $key }}: {{ $value | quote }}
    {{- end }}
spec:
  rules:
    {{- range $host := .Values.ingress.hosts }}
    - host: {{ $host }}
      http:
        paths:
          - paths: /
            backend:
              serviceName: {{ $serviceName }}
              servicePort: {{ $servicePort }}
    {{- end -}}
  {{- if .Values.ingress.tls }}
  tls:
{{ toYaml .Values.ingress.tls | indent 4 }}
  {{- end -}}
{{- end -}}
`

const ingressYAML3 = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "helloworld-chart.fullname" . }}
  labels:
    {{- include "helloworld-chart.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "helloworld-chart.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "helloworld-chart.selectorLabels" . | nindent 8 }}
    spec:
    {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
    {{- end }}
      serviceAccountName: {{ include "helloworld-chart.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: "{{ .Values.image.repository }}:{{ .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 80
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /
              port: http
          readinessProbe:
            httpGet:
              path: /
              port: http
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
    {{- end }}
    {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
    {{- end }}`

// Best practice is to NOT specify a namespace in your chart, and use --namespace
// instead, but I bet there's a user out there doing this.
const namespaceYAML = `{{- if .Values.namespace.enabled -}}
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.namespace.name }}
  labels:
    somePersistedLabel: indeed
{{- end -}}
`

const serviceYAML = `apiVersion: v1
kind: Service
metadata:
  name: {{ template "helloworld-chart.fullname" . }}
  labels:
    app: {{ template "helloworld-chart.name" . }}
    chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
    release: {{ .Release.Name }}
    heritage: {{ .Release.Service }}
    namespaceLabel: {{ .Release.Namespace }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.externalPort }}
      targetPort: {{ .Values.service.internalPort }}
      protocol: TCP
      name: {{ .Values.service.name }}
  selector:
    app: {{ template "helloworld-chart.name" . }}
    release: {{ .Release.Name }}
`

const serviceYAML3 = `apiVersion: v1
kind: Service
metadata:
  name: {{ include "helloworld-chart.fullname" . }}
  labels:
    {{- include "helloworld-chart.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
		{{- include "helloworld-chart.selectorLabels" . | nindent 4 }}`

const serviceAccountYAML3 = `{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "helloworld-chart.serviceAccountName" . }}
  labels:
{{ include "helloworld-chart.labels" . | nindent 4 }}
{{- end -}}`

const helmTestYAML = `apiVersion: v1
kind: Pod
metadata:
  name: "{{ .Release.Name }}-credentials-test"
  namespace: {{ .Release.Namespace }}
  annotations:
    "helm.sh/hook": test-success
spec:
  containers:
  - name: {{ .Release.Name }}-credentials-test
    image: foobar
    env:
      - name: MARIADB_HOST
        value: name
      - name: MARIADB_PORT
        value: "3306"
      - name: WORDPRESS_DATABASE_NAME
        value: database
      - name: WORDPRESS_DATABASE_USER
        value: user
      - name: WORDPRESS_DATABASE_PASSWORD
        valueFrom:
          secretKeyRef:
            name: name
            key: mariadb-password
    command: ["sh", "-c", "mysql --host=$MARIADB_HOST --port=$MARIADB_PORT --user=$WORDPRESS_DATABASE_USER --password=$WORDPRESS_DATABASE_PASSWORD"]
  restartPolicy: Never
`
