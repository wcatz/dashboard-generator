package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// YAMLEditor provides structured editing of the YAML config file using
// the yaml.v3 Node API, preserving comments and formatting.
type YAMLEditor struct {
	path string
}

// NewYAMLEditor creates a new editor for the given config file path.
func NewYAMLEditor(path string) *YAMLEditor {
	return &YAMLEditor{path: path}
}

// AddDatasource adds a new datasource entry to the config file.
func (e *YAMLEditor) AddDatasource(name string, ds DatasourceDef) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dsNode := findMappingKey(root, "datasources")
	if dsNode == nil {
		// No datasources section — create one
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "datasources"},
			&yaml.Node{Kind: yaml.MappingNode},
		)
		dsNode = root.Content[len(root.Content)-1]
	}

	// Check for duplicate name
	if findMappingKey(dsNode, name) != nil {
		return fmt.Errorf("datasource '%s' already exists", name)
	}

	// Build the value node for the new datasource
	valueNode := &yaml.Node{Kind: yaml.MappingNode}
	valueNode.Content = append(valueNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: ds.Type},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "uid"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: ds.UID},
	)
	if ds.URL != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: ds.URL},
		)
	}
	if ds.IsDefault {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "is_default"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
		)
	}

	dsNode.Content = append(dsNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		valueNode,
	)

	return e.save(doc)
}

// DeleteDatasource removes a datasource entry from the config file.
func (e *YAMLEditor) DeleteDatasource(name string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dsNode := findMappingKey(root, "datasources")
	if dsNode == nil {
		return fmt.Errorf("no datasources section in config")
	}

	idx := findMappingKeyIndex(dsNode, name)
	if idx < 0 {
		return fmt.Errorf("datasource '%s' not found", name)
	}

	// Remove the key-value pair (2 consecutive entries in Content)
	dsNode.Content = append(dsNode.Content[:idx], dsNode.Content[idx+2:]...)

	return e.save(doc)
}

// UpdateDatasourceURL updates or inserts the url field for a datasource.
func (e *YAMLEditor) UpdateDatasourceURL(name, url string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dsNode := findMappingKey(root, "datasources")
	if dsNode == nil {
		return fmt.Errorf("no datasources section in config")
	}

	entryNode := findMappingKey(dsNode, name)
	if entryNode == nil {
		return fmt.Errorf("datasource '%s' not found", name)
	}

	// Find or create the url field
	urlVal := findMappingKey(entryNode, "url")
	if urlVal != nil {
		urlVal.Value = url
	} else {
		entryNode.Content = append(entryNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: url},
		)
	}

	return e.save(doc)
}

func (e *YAMLEditor) load() (*yaml.Node, *yaml.Node, error) {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parsing config: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil, fmt.Errorf("invalid YAML document")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("root is not a mapping")
	}

	return &doc, root, nil
}

func (e *YAMLEditor) save(doc *yaml.Node) error {
	out, err := os.Create(e.path)
	if err != nil {
		return fmt.Errorf("opening config for write: %w", err)
	}
	defer out.Close()

	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return enc.Close()
}

// findMappingKey finds the value node for a key in a MappingNode.
func findMappingKey(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// findMappingKeyIndex returns the index of a key in a MappingNode's Content, or -1.
func findMappingKeyIndex(mapping *yaml.Node, key string) int {
	if mapping.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}
