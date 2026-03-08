package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GrafanaDashboardCRD represents a Grafana Operator v1beta1 Dashboard resource
type GrafanaDashboardCRD struct {
	APIVersion string               `yaml:"apiVersion"`
	Kind       string               `yaml:"kind"`
	Metadata   GrafanaMetadata      `yaml:"metadata"`
	Spec       GrafanaDashboardSpec `yaml:"spec"`
}

type GrafanaMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type GrafanaDashboardSpec struct {
	InstanceSelector map[string]interface{} `yaml:"instanceSelector,omitempty"`
	Folder           string                 `yaml:"folder,omitempty"`
	JSON             string                 `yaml:"json"`
}

// WriteGrafanaYAML writes a dashboard as Grafana Operator CRD YAML
func WriteGrafanaYAML(dashboard map[string]interface{}, outputPath string, namespace string, folder string) error {
	// Marshal dashboard to JSON string (for embedding in CRD)
	dashboardJSON, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling dashboard JSON: %w", err)
	}

	// Extract dashboard metadata
	uid, _ := dashboard["uid"].(string)
	title, _ := dashboard["title"].(string)
	tags, _ := dashboard["tags"].([]interface{})

	// Create CRD name from UID (k8s-safe)
	name := sanitizeK8sName(uid)
	if name == "" {
		name = sanitizeK8sName(title)
	}

	// Convert tags to labels
	labels := make(map[string]string)
	labels["grafana_dashboard"] = "1"
	for _, tag := range tags {
		if tagStr, ok := tag.(string); ok {
			// Sanitize tag for k8s label
			labelKey := sanitizeK8sLabel(tagStr)
			labels[labelKey] = "true"
		}
	}

	// Build CRD
	crd := GrafanaDashboardCRD{
		APIVersion: "grafana.integreatly.org/v1beta1",
		Kind:       "GrafanaDashboard",
		Metadata: GrafanaMetadata{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: GrafanaDashboardSpec{
			InstanceSelector: map[string]interface{}{
				"matchLabels": map[string]string{
					"dashboards": "grafana",
				},
			},
			Folder: folder,
			JSON:   string(dashboardJSON),
		},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&crd)
	if err != nil {
		return fmt.Errorf("marshaling CRD to YAML: %w", err)
	}

	// Write to file
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("writing YAML file: %w", err)
	}

	return nil
}

// WriteConfigMap writes a dashboard as Kubernetes ConfigMap
func WriteConfigMap(dashboard map[string]interface{}, outputPath string, namespace string) error {
	// Marshal dashboard to JSON
	dashboardJSON, err := json.MarshalIndent(dashboard, "    ", "  ")
	if err != nil {
		return fmt.Errorf("marshaling dashboard JSON: %w", err)
	}

	// Extract metadata
	uid, _ := dashboard["uid"].(string)

	// Create ConfigMap name
	name := sanitizeK8sName(uid) + "-dashboard"

	// Build ConfigMap YAML
	configMap := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				"grafana_dashboard": "1",
			},
		},
		"data": map[string]string{
			uid + ".json": string(dashboardJSON),
		},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&configMap)
	if err != nil {
		return fmt.Errorf("marshaling ConfigMap to YAML: %w", err)
	}

	// Write to file
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("writing YAML file: %w", err)
	}

	return nil
}

// sanitizeK8sName converts a string to a valid Kubernetes resource name
func sanitizeK8sName(s string) string {
	// Lowercase
	s = strings.ToLower(s)
	// Replace invalid characters with hyphens
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)
	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")
	// Limit length to 63 characters (k8s limit)
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

// sanitizeK8sLabel converts a string to a valid Kubernetes label key
func sanitizeK8sLabel(s string) string {
	// Lowercase
	s = strings.ToLower(s)
	// Replace spaces and invalid characters
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, s)
	// Remove leading/trailing non-alphanumeric
	s = strings.Trim(s, "-_.")
	// Limit length
	if len(s) > 63 {
		s = s[:63]
	}
	// Prefix if starts with number
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		s = "tag-" + s
	}
	return s
}
