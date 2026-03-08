package server

import (
	"fmt"
	"strings"
)

// ConfigTemplate represents a starter configuration template
type ConfigTemplate struct {
	Name        string
	Description string
	Content     string
	Category    string // "starter", "kubernetes", "cardano", "custom"
}

// GetConfigTemplates returns all available config templates
func GetConfigTemplates() []ConfigTemplate {
	return []ConfigTemplate{
		{
			Name:        "Minimal",
			Description: "Bare minimum config with one datasource and simple dashboard",
			Category:    "starter",
			Content:     minimalTemplate,
		},
		{
			Name:        "Kubernetes Full",
			Description: "Complete K8s monitoring with pods, nodes, and resource metrics",
			Category:    "kubernetes",
			Content:     k8sFullTemplate,
		},
		{
			Name:        "Kubernetes Basic",
			Description: "Simple K8s monitoring with essential metrics only",
			Category:    "kubernetes",
			Content:     k8sBasicTemplate,
		},
		{
			Name:        "Cardano Node",
			Description: "Cardano blockchain node monitoring dashboards",
			Category:    "cardano",
			Content:     cardanoTemplate,
		},
		{
			Name:        "Infrastructure",
			Description: "System metrics: CPU, memory, disk, network",
			Category:    "infrastructure",
			Content:     infraTemplate,
		},
		{
			Name:        "Application Metrics",
			Description: "Generic application monitoring template",
			Category:    "application",
			Content:     appTemplate,
		},
	}
}

// GetTemplateByName returns a specific template by name
func GetTemplateByName(name string) (*ConfigTemplate, error) {
	templates := GetConfigTemplates()
	for _, t := range templates {
		if strings.EqualFold(t.Name, name) {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template '%s' not found", name)
}

// Template definitions

const minimalTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./output
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus-uid
    url: http://prometheus:9090
    is_default: true

palettes:
  default:
    blue: "#5794F2"
    green: "#73BF69"
    yellow: "#F2CC0C"
    red: "#F2495C"
    purple: "#B877D9"

active_palette: default

variables:
  interval:
    type: interval
    label: interval
    values: "1m,5m,10m,30m,1h"

dashboards:
  overview:
    uid: overview
    title: overview
    filename: overview.json
    tags: [monitoring]
    variables: [interval]
    sections:
      - title: "metrics"
        panels:
          - type: stat
            title: "up targets"
            query: 'count(up == 1)'
            
          - type: timeseries
            title: "cpu usage"
            query: 'rate(process_cpu_seconds_total[5m])'
            unit: percentunit
`

const k8sBasicTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./output
  editable: true

datasources:
  k8s-prometheus:
    type: prometheus
    uid: k8s-prom-uid
    url: http://prometheus-k8s:9090
    is_default: true

palettes:
  default:
    blue: "#5794F2"
    green: "#73BF69"
    yellow: "#F2CC0C"
    red: "#F2495C"
    orange: "#FF9830"

active_palette: default

thresholds:
  percent_usage:
    - color: $green
      value: 0
    - color: $yellow
      value: 70
    - color: $red
      value: 90

variables:
  namespace:
    type: query
    datasource: k8s-prometheus
    query: 'label_values(kube_pod_info, namespace)'
    label: namespace
    multi: true
    include_all: true
    
  pod:
    type: query
    datasource: k8s-prometheus
    query: 'label_values(kube_pod_info{namespace="$namespace"}, pod)'
    label: pod
    multi: true
    include_all: true

dashboards:
  k8s_overview:
    uid: k8s-overview
    title: kubernetes overview
    filename: k8s-overview.json
    tags: [kubernetes, overview]
    icon: apps
    variables: [namespace, pod]
    sections:
      - title: "cluster health"
        panels:
          - type: stat
            title: "namespaces"
            query: 'count(count by (namespace) (kube_pod_info))'
            color: $blue
            
          - type: stat
            title: "pods running"
            query: 'sum(kube_pod_status_phase{phase="Running"})'
            color: $green
            
          - type: stat
            title: "pods pending"
            query: 'sum(kube_pod_status_phase{phase="Pending"})'
            color: $yellow
            
          - type: stat
            title: "pods failed"
            query: 'sum(kube_pod_status_phase{phase="Failed"})'
            color: $red
            
      - title: "resource usage"
        panels:
          - type: timeseries
            title: "cpu usage by namespace"
            query: 'sum(rate(container_cpu_usage_seconds_total{namespace="$namespace"}[5m])) by (namespace)'
            unit: cores
            
          - type: timeseries
            title: "memory usage by namespace"
            query: 'sum(container_memory_working_set_bytes{namespace="$namespace"}) by (namespace)'
            unit: bytes
`

const k8sFullTemplate = `# Full template - see example-config.yaml for complete version
generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./output

datasources:
  k8s-prometheus:
    type: prometheus
    uid: k8s-prom
    url: http://prometheus-k8s:9090
    is_default: true

# ... Add full example-config.yaml content here for complete K8s monitoring
`

const cardanoTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 6h
  output_dir: ./output
  editable: true

datasources:
  cardano-prometheus:
    type: prometheus
    uid: cardano-prom
    url: http://prometheus:9090
    is_default: true

palettes:
  cardano:
    blue: "#0033AD"
    cyan: "#00D4FF"
    green: "#45B26B"
    yellow: "#FFB800"
    red: "#FF5757"
    purple: "#7B64FF"

active_palette: cardano

variables:
  network:
    type: query
    datasource: cardano-prometheus
    query: 'label_values(cardano_node_metrics_Forge_forge_about_to_lead_int, network)'
    label: network
    
  alias:
    type: query
    datasource: cardano-prometheus
    query: 'label_values(cardano_node_metrics_Forge_forge_about_to_lead_int{network="$network"}, alias)'
    label: node
    multi: true
    include_all: true

dashboards:
  cardano_overview:
    uid: cardano-overview
    title: cardano node overview
    filename: cardano-overview.json
    tags: [cardano, blockchain]
    icon: exchange-alt
    variables: [network, alias]
    sections:
      - title: "chain sync"
        panels:
          - type: gauge
            title: "sync progress"
            query: 'cardano_node_metrics_ChainDB_metrics_slotNum_int / cardano_node_metrics_ChainDB_metrics_slotInEpoch_int * 100'
            unit: percent
            min: 0
            max: 100
            
          - type: stat
            title: "current epoch"
            query: 'cardano_node_metrics_ChainDB_metrics_epoch_int'
            
          - type: timeseries
            title: "blocks processed"
            query: 'rate(cardano_node_metrics_ChainDB_metrics_blockNum_int[5m])'
`

const infraTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./output

datasources:
  node-exporter:
    type: prometheus
    uid: node-prom
    url: http://prometheus:9090
    is_default: true

palettes:
  default:
    blue: "#5794F2"
    green: "#73BF69"
    yellow: "#F2CC0C"
    red: "#F2495C"

active_palette: default

thresholds:
  percent_usage:
    - color: $green
      value: 0
    - color: $yellow
      value: 80
    - color: $red
      value: 95

variables:
  instance:
    type: query
    query: 'label_values(node_uname_info, instance)'
    label: instance
    multi: true
    include_all: true

dashboards:
  system_overview:
    uid: system-overview
    title: system overview
    filename: system-overview.json
    tags: [infrastructure]
    variables: [instance]
    sections:
      - title: "cpu"
        panels:
          - type: gauge
            title: "cpu usage"
            query: '100 - (avg(rate(node_cpu_seconds_total{mode="idle",instance=~"$instance"}[5m])) * 100)'
            unit: percent
            thresholds: $percent_usage
            
      - title: "memory"
        panels:
          - type: gauge
            title: "memory usage"
            query: '(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100'
            unit: percent
            thresholds: $percent_usage
`

const appTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./output

datasources:
  app-prometheus:
    type: prometheus
    uid: app-prom
    url: http://prometheus:9090
    is_default: true

palettes:
  default:
    blue: "#5794F2"
    green: "#73BF69"
    yellow: "#F2CC0C"
    red: "#F2495C"

active_palette: default

variables:
  job:
    type: query
    query: 'label_values(up, job)'
    label: application
    multi: true
    include_all: true
    
  interval:
    type: interval
    values: "1m,5m,10m,30m,1h"
    label: interval

dashboards:
  app_overview:
    uid: app-overview
    title: application overview
    filename: app-overview.json
    tags: [application]
    variables: [job, interval]
    sections:
      - title: "health"
        panels:
          - type: stat
            title: "uptime"
            query: 'avg(up{job=~"$job"})'
            unit: short
            
          - type: timeseries
            title: "request rate"
            query: 'rate(http_requests_total{job=~"$job"}[$interval])'
            
          - type: timeseries
            title: "error rate"
            query: 'rate(http_requests_total{job=~"$job",status=~"5.."}[$interval])'
`
