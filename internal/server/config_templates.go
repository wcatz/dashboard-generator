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
	Category    string // "starter", "infrastructure", "kubernetes", "application"
}

// GetConfigTemplates returns all available config templates
func GetConfigTemplates() []ConfigTemplate {
	return []ConfigTemplate{
		{
			Name:        "Blank",
			Description: "Empty project — just a datasource placeholder to start from scratch",
			Category:    "starter",
			Content:     blankTemplate,
		},
		{
			Name:        "Node Exporter",
			Description: "Host metrics via node_exporter — CPU, memory, disk, filesystem, network, load, uptime",
			Category:    "infrastructure",
			Content:     nodeExporterTemplate,
		},
		{
			Name:        "Kubernetes Cluster",
			Description: "Kubernetes cluster overview — namespaces, pods, nodes, resource usage",
			Category:    "kubernetes",
			Content:     k8sClusterTemplate,
		},
		{
			Name:        "Kubernetes Workloads",
			Description: "Kubernetes workloads — deployments, replicas, HPA, container resources",
			Category:    "kubernetes",
			Content:     k8sWorkloadsTemplate,
		},
		{
			Name:        "NGINX Ingress",
			Description: "NGINX Ingress Controller — request rates, latency, errors, connections",
			Category:    "application",
			Content:     nginxIngressTemplate,
		},
		{
			Name:        "Generic Application",
			Description: "Application RED metrics — request rate, errors, duration, Go runtime",
			Category:    "application",
			Content:     genericAppTemplate,
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

const blankTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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
  blank:
    uid: blank
    title: blank
    filename: blank.json
    tags: [generated]
    variables: [interval]
    sections:
      - title: "metrics"
        panels:
          - type: stat
            title: "up targets"
            query: 'up'
`

const nodeExporterTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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

thresholds:
  percent_usage:
    - {color: "$green", value: 0}
    - {color: "$yellow", value: 70}
    - {color: "$red", value: 90}
  percent_usage_inverse:
    - {color: "$red", value: 0}
    - {color: "$yellow", value: 30}
    - {color: "$green", value: 60}

variables:
  instance:
    type: query
    query: 'label_values(node_uname_info, instance)'
    label: instance
  interval:
    type: interval
    label: interval
    values: "1m,5m,10m,30m,1h"

dashboards:
  node_exporter:
    uid: node-exporter
    title: node exporter
    filename: node-exporter.json
    tags: [node-exporter, infrastructure, generated]
    variables: [instance, interval]
    sections:
      - title: "overview"
        panels:
          - type: stat
            title: "uptime"
            query: 'time()-node_boot_time_seconds'
            width: 4
          - type: gauge
            title: "CPU usage"
            query: '100-(avg by(instance)(irate(node_cpu_seconds_total{mode="idle",instance=~"$instance"}[5m]))*100)'
            unit: percent
            thresholds: $percent_usage
            width: 5
          - type: gauge
            title: "memory usage"
            query: '(1-(node_memory_MemAvailable_bytes{instance=~"$instance"}/node_memory_MemTotal_bytes{instance=~"$instance"}))*100'
            unit: percent
            thresholds: $percent_usage
            width: 5
          - type: gauge
            title: "root filesystem"
            query: '100-(node_filesystem_avail_bytes{instance=~"$instance",mountpoint="/",fstype!="rootfs"}/node_filesystem_size_bytes{instance=~"$instance",mountpoint="/",fstype!="rootfs"})*100'
            unit: percent
            thresholds: $percent_usage
            width: 5
          - type: stat
            title: "load 1m"
            query: 'node_load1{instance=~"$instance"}'
            width: 5
      - title: "cpu"
        panels:
          - type: timeseries
            title: "CPU by mode"
            query: 'irate(node_cpu_seconds_total{instance=~"$instance"}[$interval])'
            legend: "{{mode}}"
            unit: percentunit
            draw_style: line
            fill_opacity: 30
            stack: normal
          - type: timeseries
            title: "system load"
            unit: short
            targets:
              - expr: 'node_load1{instance=~"$instance"}'
                legend: "1m"
              - expr: 'node_load5{instance=~"$instance"}'
                legend: "5m"
              - expr: 'node_load15{instance=~"$instance"}'
                legend: "15m"
      - title: "memory"
        panels:
          - type: timeseries
            title: "memory breakdown"
            unit: bytes
            targets:
              - expr: 'node_memory_MemTotal_bytes{instance=~"$instance"}'
                legend: "total"
              - expr: 'node_memory_MemTotal_bytes{instance=~"$instance"}-node_memory_MemAvailable_bytes{instance=~"$instance"}'
                legend: "used"
              - expr: 'node_memory_MemAvailable_bytes{instance=~"$instance"}'
                legend: "available"
              - expr: 'node_memory_Cached_bytes{instance=~"$instance"}'
                legend: "cached"
              - expr: 'node_memory_Buffers_bytes{instance=~"$instance"}'
                legend: "buffers"
          - type: stat
            title: "swap used"
            query: 'node_memory_SwapTotal_bytes{instance=~"$instance"}-node_memory_SwapFree_bytes{instance=~"$instance"}'
            unit: bytes
      - title: "disk"
        panels:
          - type: timeseries
            title: "disk I/O read"
            query: 'irate(node_disk_read_bytes_total{instance=~"$instance"}[$interval])'
            unit: Bps
            width: 8
          - type: timeseries
            title: "disk I/O write"
            query: 'irate(node_disk_written_bytes_total{instance=~"$instance"}[$interval])'
            unit: Bps
            width: 8
          - type: timeseries
            title: "disk IO time"
            query: 'irate(node_disk_io_time_seconds_total{instance=~"$instance"}[$interval])'
            unit: percentunit
            width: 8
      - title: "network"
        panels:
          - type: timeseries
            title: "received"
            query: 'irate(node_network_receive_bytes_total{instance=~"$instance",device!~"lo|veth.*|docker.*|br.*"}[$interval])'
            legend: "{{device}} rx"
            unit: Bps
          - type: timeseries
            title: "transmitted"
            query: 'irate(node_network_transmit_bytes_total{instance=~"$instance",device!~"lo|veth.*|docker.*|br.*"}[$interval])'
            legend: "{{device}} tx"
            unit: Bps
      - title: "filesystem"
        panels:
          - type: table
            title: "filesystems"
            query: 'node_filesystem_size_bytes{instance=~"$instance",fstype!~"tmpfs|overlay"}'
            description: "filesystem capacity overview"
`

const k8sClusterTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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

thresholds:
  percent_usage:
    - {color: "$green", value: 0}
    - {color: "$yellow", value: 70}
    - {color: "$red", value: 90}

variables:
  namespace:
    type: query
    query: 'label_values(kube_pod_info,namespace)'
    label: namespace
    multi: true
    include_all: true
  pod:
    type: query
    query: 'label_values(kube_pod_info{namespace=~"$namespace"},pod)'
    label: pod
    multi: true
    include_all: true
    chains_from: [namespace]

dashboards:
  k8s_cluster:
    uid: k8s-cluster
    title: kubernetes cluster
    filename: k8s-cluster.json
    tags: [kubernetes, cluster, generated]
    variables: [namespace, pod]
    sections:
      - title: "cluster overview"
        panels:
          - type: stat
            title: "namespaces"
            query: 'count(count by(namespace)(kube_pod_info))'
            width: 4
            color: $blue
          - type: stat
            title: "nodes ready"
            query: 'sum(kube_node_status_condition{condition="Ready",status="true"})'
            width: 5
            color: $green
          - type: stat
            title: "pods running"
            query: 'sum(kube_pod_status_phase{namespace=~"$namespace",phase="Running"})'
            width: 5
            color: $green
          - type: stat
            title: "pods pending"
            query: 'sum(kube_pod_status_phase{namespace=~"$namespace",phase="Pending"})'
            width: 5
            color: $yellow
          - type: stat
            title: "pods failed"
            query: 'sum(kube_pod_status_phase{namespace=~"$namespace",phase="Failed"})'
            width: 5
            color: $red
      - title: "resource usage"
        panels:
          - type: gauge
            title: "cluster CPU"
            query: 'sum(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",container!="",container!="POD"}[5m]))/sum(kube_node_status_allocatable{resource="cpu"})*100'
            unit: percent
            thresholds: $percent_usage
            width: 12
          - type: gauge
            title: "cluster memory"
            query: 'sum(container_memory_working_set_bytes{namespace=~"$namespace",container!="",container!="POD"})/sum(kube_node_status_allocatable{resource="memory"})*100'
            unit: percent
            thresholds: $percent_usage
            width: 12
      - title: "cpu by namespace"
        panels:
          - type: timeseries
            title: "CPU usage by namespace"
            query: 'sum by(namespace)(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",container!="",container!="POD"}[5m]))'
            legend: "{{namespace}}"
            unit: cores
            stack: normal
            fill_opacity: 30
      - title: "memory by namespace"
        panels:
          - type: timeseries
            title: "memory usage by namespace"
            query: 'sum by(namespace)(container_memory_working_set_bytes{namespace=~"$namespace",container!="",container!="POD"})'
            legend: "{{namespace}}"
            unit: bytes
            stack: normal
            fill_opacity: 30
      - title: "pod health"
        panels:
          - type: timeseries
            title: "pod restarts"
            query: 'sum by(namespace,pod)(increase(kube_pod_container_status_restarts_total{namespace=~"$namespace"}[1h]))'
            legend: "{{namespace}}/{{pod}}"
          - type: stat
            title: "OOM kills"
            query: 'sum(increase(kube_pod_container_status_last_terminated_reason{reason="OOMKilled",namespace=~"$namespace"}[24h]))'
            color: $red
            description: "last 24h"
`

const k8sWorkloadsTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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
  namespace:
    type: query
    query: 'label_values(kube_pod_info,namespace)'
    label: namespace
    multi: true
    include_all: true
  deployment:
    type: query
    query: 'label_values(kube_deployment_created{namespace=~"$namespace"},deployment)'
    label: deployment
    multi: true
    include_all: true
    chains_from: [namespace]

dashboards:
  k8s_workloads:
    uid: k8s-workloads
    title: kubernetes workloads
    filename: k8s-workloads.json
    tags: [kubernetes, workloads, generated]
    variables: [namespace, deployment]
    sections:
      - title: "deployments"
        panels:
          - type: stat
            title: "total deployments"
            query: 'count(kube_deployment_created{namespace=~"$namespace"})'
            width: 8
          - type: stat
            title: "available replicas"
            query: 'sum(kube_deployment_status_replicas_available{namespace=~"$namespace",deployment=~"$deployment"})'
            width: 8
            color: $green
          - type: stat
            title: "unavailable replicas"
            query: 'sum(kube_deployment_status_replicas_unavailable{namespace=~"$namespace",deployment=~"$deployment"})'
            width: 8
            color: $red
      - title: "replica status"
        panels:
          - type: timeseries
            title: "desired vs available"
            targets:
              - expr: 'kube_deployment_spec_replicas{namespace=~"$namespace",deployment=~"$deployment"}'
                legend: "{{deployment}} desired"
              - expr: 'kube_deployment_status_replicas_available{namespace=~"$namespace",deployment=~"$deployment"}'
                legend: "{{deployment}} available"
      - title: "container resources"
        panels:
          - type: timeseries
            title: "CPU requests vs actual"
            unit: cores
            targets:
              - expr: 'sum by(container)(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",container!="",container!="POD"}[5m]))'
                legend: "{{container}} actual"
              - expr: 'sum by(container)(kube_pod_container_resource_requests{namespace=~"$namespace",resource="cpu"})'
                legend: "{{container}} request"
          - type: timeseries
            title: "memory requests vs actual"
            unit: bytes
            targets:
              - expr: 'sum by(container)(container_memory_working_set_bytes{namespace=~"$namespace",container!="",container!="POD"})'
                legend: "{{container}} actual"
              - expr: 'sum by(container)(kube_pod_container_resource_requests{namespace=~"$namespace",resource="memory"})'
                legend: "{{container}} request"
      - title: "HPA"
        panels:
          - type: timeseries
            title: "HPA replicas"
            draw_style: line
            targets:
              - expr: 'kube_horizontalpodautoscaler_status_current_replicas{namespace=~"$namespace"}'
                legend: "{{horizontalpodautoscaler}}"
              - expr: 'kube_horizontalpodautoscaler_spec_max_replicas{namespace=~"$namespace"}'
                legend: "{{horizontalpodautoscaler}} max"
`

const nginxIngressTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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
  namespace:
    type: query
    query: 'label_values(nginx_ingress_controller_requests,namespace)'
    label: namespace
    multi: true
    include_all: true
  ingress:
    type: query
    query: 'label_values(nginx_ingress_controller_requests{namespace=~"$namespace"},ingress)'
    label: ingress
    multi: true
    include_all: true
    chains_from: [namespace]
  interval:
    type: interval
    label: interval
    values: "1m,5m,10m,30m,1h"

dashboards:
  nginx_ingress:
    uid: nginx-ingress
    title: nginx ingress
    filename: nginx-ingress.json
    tags: [nginx, ingress, generated]
    variables: [namespace, ingress, interval]
    sections:
      - title: "overview"
        panels:
          - type: stat
            title: "request rate"
            query: 'sum(rate(nginx_ingress_controller_requests{namespace=~"$namespace",ingress=~"$ingress"}[$interval]))'
            unit: reqps
            width: 8
          - type: stat
            title: "error rate 5xx"
            query: 'sum(rate(nginx_ingress_controller_requests{namespace=~"$namespace",ingress=~"$ingress",status=~"5.."}[$interval]))'
            unit: reqps
            color: $red
            width: 8
          - type: stat
            title: "active connections"
            query: 'sum(nginx_ingress_controller_nginx_process_connections{state="active"})'
            width: 8
      - title: "traffic"
        panels:
          - type: timeseries
            title: "requests by status"
            query: 'sum by(status)(rate(nginx_ingress_controller_requests{namespace=~"$namespace",ingress=~"$ingress"}[$interval]))'
            legend: "{{status}}"
            unit: reqps
            stack: normal
            fill_opacity: 30
          - type: timeseries
            title: "requests by ingress"
            query: 'sum by(ingress)(rate(nginx_ingress_controller_requests{namespace=~"$namespace",ingress=~"$ingress"}[$interval]))'
            legend: "{{ingress}}"
            unit: reqps
      - title: "latency"
        panels:
          - type: timeseries
            title: "response time percentiles"
            unit: s
            targets:
              - expr: 'histogram_quantile(0.50,sum by(le)(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace=~"$namespace",ingress=~"$ingress"}[$interval])))'
                legend: "p50"
              - expr: 'histogram_quantile(0.90,sum by(le)(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace=~"$namespace",ingress=~"$ingress"}[$interval])))'
                legend: "p90"
              - expr: 'histogram_quantile(0.99,sum by(le)(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace=~"$namespace",ingress=~"$ingress"}[$interval])))'
                legend: "p99"
          - type: timeseries
            title: "upstream response time"
            query: 'histogram_quantile(0.95,sum by(le,ingress)(rate(nginx_ingress_controller_response_duration_seconds_bucket{namespace=~"$namespace",ingress=~"$ingress"}[$interval])))'
            legend: "{{ingress}} p95"
            unit: s
      - title: "connections"
        panels:
          - type: timeseries
            title: "connections by state"
            fill_opacity: 20
            targets:
              - expr: 'nginx_ingress_controller_nginx_process_connections{state="active"}'
                legend: "active"
              - expr: 'nginx_ingress_controller_nginx_process_connections{state="reading"}'
                legend: "reading"
              - expr: 'nginx_ingress_controller_nginx_process_connections{state="writing"}'
                legend: "writing"
              - expr: 'nginx_ingress_controller_nginx_process_connections{state="waiting"}'
                legend: "waiting"
`

const genericAppTemplate = `generator:
  schema_version: 39
  refresh: 30s
  time_range: 1h
  output_dir: ./dashboards
  editable: true

datasources:
  prometheus:
    type: prometheus
    uid: prometheus
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
  job:
    type: query
    query: 'label_values(up,job)'
    label: job
    multi: true
    include_all: true
  interval:
    type: interval
    label: interval
    values: "1m,5m,10m,30m,1h"

dashboards:
  app_overview:
    uid: app-overview
    title: application overview
    filename: app-overview.json
    tags: [application, red-metrics, generated]
    variables: [job, interval]
    sections:
      - title: "health"
        panels:
          - type: stat
            title: "uptime"
            query: 'avg(time()-process_start_time_seconds{job=~"$job"})'
            unit: s
            width: 8
          - type: stat
            title: "up instances"
            query: 'count(up{job=~"$job"}==1)'
            color: $green
            width: 8
          - type: stat
            title: "total requests"
            query: 'sum(rate(http_requests_total{job=~"$job"}[$interval]))'
            unit: reqps
            width: 8
      - title: "RED metrics"
        panels:
          - type: timeseries
            title: "request rate"
            query: 'sum by(handler)(rate(http_requests_total{job=~"$job"}[$interval]))'
            legend: "{{handler}}"
            unit: reqps
            width: 8
          - type: timeseries
            title: "error rate"
            query: 'sum by(handler)(rate(http_requests_total{job=~"$job",code=~"5.."}[$interval]))'
            legend: "{{handler}}"
            unit: reqps
            color: $red
            width: 8
          - type: timeseries
            title: "request duration"
            unit: s
            width: 8
            targets:
              - expr: 'histogram_quantile(0.95,sum by(le)(rate(http_request_duration_seconds_bucket{job=~"$job"}[$interval])))'
                legend: "p95"
              - expr: 'histogram_quantile(0.50,sum by(le)(rate(http_request_duration_seconds_bucket{job=~"$job"}[$interval])))'
                legend: "p50"
      - title: "Go runtime"
        panels:
          - type: timeseries
            title: "goroutines"
            query: 'go_goroutines{job=~"$job"}'
            legend: "{{instance}}"
            width: 8
          - type: timeseries
            title: "GC duration"
            query: 'rate(go_gc_duration_seconds_sum{job=~"$job"}[$interval])'
            legend: "{{instance}}"
            unit: s
            width: 8
          - type: stat
            title: "memory alloc"
            query: 'go_memstats_alloc_bytes{job=~"$job"}'
            unit: bytes
            width: 8
`
