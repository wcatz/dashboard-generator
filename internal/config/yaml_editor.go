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
	if ds.BasicUser != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "basic_user"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: ds.BasicUser},
		)
	}
	if ds.BasicPass != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "basic_pass"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: ds.BasicPass},
		)
	}
	if ds.Token != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "token"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: ds.Token},
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

// UpdateDatasourceAuth updates auth fields for an existing datasource.
// Empty values remove the corresponding field.
func (e *YAMLEditor) UpdateDatasourceAuth(name, basicUser, basicPass, token string) error {
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

	setOrRemoveField(entryNode, "basic_user", basicUser)
	setOrRemoveField(entryNode, "basic_pass", basicPass)
	setOrRemoveField(entryNode, "token", token)

	return e.save(doc)
}

// setOrRemoveField sets a scalar field on a mapping node, or removes it if value is empty.
func setOrRemoveField(node *yaml.Node, key, value string) {
	if value != "" {
		existing := findMappingKey(node, key)
		if existing != nil {
			existing.Value = value
		} else {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: key},
				&yaml.Node{Kind: yaml.ScalarNode, Value: value},
			)
		}
	} else {
		idx := findMappingKeyIndex(node, key)
		if idx >= 0 {
			node.Content = append(node.Content[:idx], node.Content[idx+2:]...)
		}
	}
}

// SetPaletteColor sets or updates a color in a named palette.
func (e *YAMLEditor) SetPaletteColor(palette, color, hex string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil {
		return fmt.Errorf("no palettes section in config")
	}

	paletteNode := findMappingKey(palettesNode, palette)
	if paletteNode == nil {
		return fmt.Errorf("palette '%s' not found", palette)
	}

	colorVal := findMappingKey(paletteNode, color)
	if colorVal != nil {
		colorVal.Value = hex
		colorVal.Style = yaml.DoubleQuotedStyle
	} else {
		paletteNode.Content = append(paletteNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: color},
			&yaml.Node{Kind: yaml.ScalarNode, Value: hex, Style: yaml.DoubleQuotedStyle},
		)
	}

	return e.save(doc)
}

// DeletePaletteColor removes a color from a named palette.
func (e *YAMLEditor) DeletePaletteColor(palette, color string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil {
		return fmt.Errorf("no palettes section in config")
	}

	paletteNode := findMappingKey(palettesNode, palette)
	if paletteNode == nil {
		return fmt.Errorf("palette '%s' not found", palette)
	}

	idx := findMappingKeyIndex(paletteNode, color)
	if idx < 0 {
		return fmt.Errorf("color '%s' not found in palette '%s'", color, palette)
	}

	paletteNode.Content = append(paletteNode.Content[:idx], paletteNode.Content[idx+2:]...)
	return e.save(doc)
}

// RenamePaletteColor renames a color key within a palette.
func (e *YAMLEditor) RenamePaletteColor(palette, oldName, newName string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil {
		return fmt.Errorf("no palettes section in config")
	}

	paletteNode := findMappingKey(palettesNode, palette)
	if paletteNode == nil {
		return fmt.Errorf("palette '%s' not found", palette)
	}

	idx := findMappingKeyIndex(paletteNode, oldName)
	if idx < 0 {
		return fmt.Errorf("color '%s' not found in palette '%s'", oldName, palette)
	}

	if findMappingKey(paletteNode, newName) != nil {
		return fmt.Errorf("color '%s' already exists in palette '%s'", newName, palette)
	}

	paletteNode.Content[idx].Value = newName
	return e.save(doc)
}

// AddPalette creates a new empty palette.
func (e *YAMLEditor) AddPalette(name string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "palettes"},
			&yaml.Node{Kind: yaml.MappingNode},
		)
		palettesNode = root.Content[len(root.Content)-1]
	}

	if findMappingKey(palettesNode, name) != nil {
		return fmt.Errorf("palette '%s' already exists", name)
	}

	palettesNode.Content = append(palettesNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		&yaml.Node{Kind: yaml.MappingNode},
	)

	return e.save(doc)
}

// DeletePalette removes a palette entirely.
func (e *YAMLEditor) DeletePalette(name string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil {
		return fmt.Errorf("no palettes section in config")
	}

	idx := findMappingKeyIndex(palettesNode, name)
	if idx < 0 {
		return fmt.Errorf("palette '%s' not found", name)
	}

	palettesNode.Content = append(palettesNode.Content[:idx], palettesNode.Content[idx+2:]...)
	return e.save(doc)
}

// SetActivePalette updates the active_palette key.
func (e *YAMLEditor) SetActivePalette(name string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	palettesNode := findMappingKey(root, "palettes")
	if palettesNode == nil || findMappingKey(palettesNode, name) == nil {
		return fmt.Errorf("palette '%s' not found", name)
	}

	apNode := findMappingKey(root, "active_palette")
	if apNode != nil {
		apNode.Value = name
	} else {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "active_palette"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		)
	}

	return e.save(doc)
}

// AddVariable adds a new variable entry to the config file.
func (e *YAMLEditor) AddVariable(name string, v VariableDef) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	varNode := findMappingKey(root, "variables")
	if varNode == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "variables"},
			&yaml.Node{Kind: yaml.MappingNode},
		)
		varNode = root.Content[len(root.Content)-1]
	}

	if findMappingKey(varNode, name) != nil {
		return fmt.Errorf("variable '%s' already exists", name)
	}

	valueNode := &yaml.Node{Kind: yaml.MappingNode}
	valueNode.Content = append(valueNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: v.Type},
	)
	if v.Datasource != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "datasource"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.Datasource},
		)
	}
	if v.Query != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "query"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.Query, Style: yaml.SingleQuotedStyle},
		)
	}
	if v.Values != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "values"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.Values},
		)
	}
	if v.Multi {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "multi"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
		)
	}
	if v.IncludeAll {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "include_all"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
		)
	}
	if v.Refresh > 0 {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "refresh"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v.Refresh), Tag: "!!int"},
		)
	}
	if v.Sort > 0 {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "sort"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v.Sort), Tag: "!!int"},
		)
	}
	if v.Regex != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "regex"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.Regex, Style: yaml.SingleQuotedStyle},
		)
	}
	if v.DsType != "" {
		valueNode.Content = append(valueNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "ds_type"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v.DsType},
		)
	}

	varNode.Content = append(varNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		valueNode,
	)

	return e.save(doc)
}

// AddDashboard adds a new dashboard entry to the config from YAML bytes.
// The dashboardYAML should contain the dashboard fields (uid, title, sections, etc.).
func (e *YAMLEditor) AddDashboard(name string, dashboardYAML []byte) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dashNode := findMappingKey(root, "dashboards")
	if dashNode == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "dashboards"},
			&yaml.Node{Kind: yaml.MappingNode},
		)
		dashNode = root.Content[len(root.Content)-1]
	}

	if findMappingKey(dashNode, name) != nil {
		return fmt.Errorf("dashboard '%s' already exists", name)
	}

	// Parse the dashboard YAML into a node
	wrapped := fmt.Sprintf("%s:\n", name)
	wrapped += string(dashboardYAML)
	var tmpDoc yaml.Node
	if err := yaml.Unmarshal([]byte(wrapped), &tmpDoc); err != nil {
		return fmt.Errorf("parsing dashboard YAML: %w", err)
	}

	if tmpDoc.Kind != yaml.DocumentNode || len(tmpDoc.Content) == 0 {
		return fmt.Errorf("invalid dashboard YAML structure")
	}
	tmpRoot := tmpDoc.Content[0]
	if tmpRoot.Kind != yaml.MappingNode || len(tmpRoot.Content) < 2 {
		return fmt.Errorf("dashboard YAML produced no content")
	}

	// Append the key-value pair
	dashNode.Content = append(dashNode.Content, tmpRoot.Content[0], tmpRoot.Content[1])

	return e.save(doc)
}

// AppendSection appends a section (parsed from YAML bytes) to a dashboard's sections array.
// The sectionYAML should be a valid section entry (title + panels).
func (e *YAMLEditor) AppendSection(dashboard string, sectionYAML []byte) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dashNode := findMappingKey(root, "dashboards")
	if dashNode == nil {
		return fmt.Errorf("no dashboards section in config")
	}

	dbNode := findMappingKey(dashNode, dashboard)
	if dbNode == nil {
		return fmt.Errorf("dashboard '%s' not found", dashboard)
	}

	sectionsNode := findMappingKey(dbNode, "sections")
	if sectionsNode == nil {
		// Create sections sequence if missing
		dbNode.Content = append(dbNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "sections"},
			&yaml.Node{Kind: yaml.SequenceNode},
		)
		sectionsNode = dbNode.Content[len(dbNode.Content)-1]
	}
	if sectionsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'sections' is not a sequence in dashboard '%s'", dashboard)
	}

	// Parse the section YAML into a node
	// Wrap in a sequence so yaml.v3 can parse it as a list item
	wrapped := append([]byte("sections:\n"), sectionYAML...)
	var tmpDoc yaml.Node
	if err := yaml.Unmarshal(wrapped, &tmpDoc); err != nil {
		return fmt.Errorf("parsing section YAML: %w", err)
	}

	// Navigate: Document → Mapping → "sections" value → Sequence items
	if tmpDoc.Kind != yaml.DocumentNode || len(tmpDoc.Content) == 0 {
		return fmt.Errorf("invalid section YAML structure")
	}
	tmpRoot := tmpDoc.Content[0]
	tmpSections := findMappingKey(tmpRoot, "sections")
	if tmpSections == nil || tmpSections.Kind != yaml.SequenceNode || len(tmpSections.Content) == 0 {
		return fmt.Errorf("section YAML produced no sections")
	}

	// Append all parsed sections
	sectionsNode.Content = append(sectionsNode.Content, tmpSections.Content...)

	return e.save(doc)
}

// AppendPanel appends a panel (parsed from YAML bytes) to a specific section's panels array.
// sectionIndex is 0-based.
func (e *YAMLEditor) AppendPanel(dashboard string, sectionIndex int, panelYAML []byte) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	dashNode := findMappingKey(root, "dashboards")
	if dashNode == nil {
		return fmt.Errorf("no dashboards section in config")
	}

	dbNode := findMappingKey(dashNode, dashboard)
	if dbNode == nil {
		return fmt.Errorf("dashboard '%s' not found", dashboard)
	}

	sectionsNode := findMappingKey(dbNode, "sections")
	if sectionsNode == nil || sectionsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("no sections in dashboard '%s'", dashboard)
	}

	if sectionIndex < 0 || sectionIndex >= len(sectionsNode.Content) {
		return fmt.Errorf("section index %d out of range (0-%d)", sectionIndex, len(sectionsNode.Content)-1)
	}

	sectionNode := sectionsNode.Content[sectionIndex]
	if sectionNode.Kind != yaml.MappingNode {
		return fmt.Errorf("section %d is not a mapping", sectionIndex)
	}

	panelsNode := findMappingKey(sectionNode, "panels")
	if panelsNode == nil {
		sectionNode.Content = append(sectionNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "panels"},
			&yaml.Node{Kind: yaml.SequenceNode},
		)
		panelsNode = sectionNode.Content[len(sectionNode.Content)-1]
	}
	if panelsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'panels' is not a sequence in section %d", sectionIndex)
	}

	// Parse panel YAML — wrap as sequence item
	wrapped := append([]byte("panels:\n"), panelYAML...)
	var tmpDoc yaml.Node
	if err := yaml.Unmarshal(wrapped, &tmpDoc); err != nil {
		return fmt.Errorf("parsing panel YAML: %w", err)
	}

	if tmpDoc.Kind != yaml.DocumentNode || len(tmpDoc.Content) == 0 {
		return fmt.Errorf("invalid panel YAML structure")
	}
	tmpRoot := tmpDoc.Content[0]
	tmpPanels := findMappingKey(tmpRoot, "panels")
	if tmpPanels == nil || tmpPanels.Kind != yaml.SequenceNode || len(tmpPanels.Content) == 0 {
		return fmt.Errorf("panel YAML produced no panels")
	}

	panelsNode.Content = append(panelsNode.Content, tmpPanels.Content...)

	return e.save(doc)
}

// DeleteVariable removes a variable entry from the config file.
func (e *YAMLEditor) DeleteVariable(name string) error {
	doc, root, err := e.load()
	if err != nil {
		return err
	}

	varNode := findMappingKey(root, "variables")
	if varNode == nil {
		return fmt.Errorf("no variables section in config")
	}

	idx := findMappingKeyIndex(varNode, name)
	if idx < 0 {
		return fmt.Errorf("variable '%s' not found", name)
	}

	varNode.Content = append(varNode.Content[:idx], varNode.Content[idx+2:]...)
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
