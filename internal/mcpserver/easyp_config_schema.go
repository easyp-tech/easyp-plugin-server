package mcpserver

import (
	"encoding/json"
	"sort"

	invjsonschema "github.com/invopop/jsonschema"
)

func buildEasypConfigSchemaIndex() map[string]map[string]any {
	reflector := &invjsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
	}

	schema := reflector.Reflect(easypConfigSchemaRoot{})
	root := invSchemaToMap(schema)
	if len(root) == 0 {
		return map[string]map[string]any{}
	}

	index := map[string]map[string]any{
		"$": root,
	}
	walkSchemaPaths(index, "$", root)

	return index
}

func walkSchemaPaths(index map[string]map[string]any, basePath string, schema map[string]any) {
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		branches, ok := asSchemaArray(schema[key])
		if !ok {
			continue
		}
		for _, branch := range branches {
			walkSchemaPaths(index, basePath, branch)
		}
	}

	props, ok := asSchemaMap(schema["properties"])
	if ok {
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			child, ok := asSchemaMap(props[name])
			if !ok {
				continue
			}
			childPath := joinSchemaPath(basePath, name)
			if _, exists := index[childPath]; !exists {
				index[childPath] = child
			}
			walkSchemaPaths(index, childPath, child)
		}
	}

	if items, ok := asSchemaMap(schema["items"]); ok {
		arrayPath := basePath + "[]"
		if _, exists := index[arrayPath]; !exists {
			index[arrayPath] = items
		}
		walkSchemaPaths(index, arrayPath, items)
	}
}

func joinSchemaPath(base, child string) string {
	if base == "$" {
		return child
	}
	return base + "." + child
}

func asSchemaArray(v any) ([]map[string]any, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := asSchemaMap(item)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func asSchemaMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return m, true
}

func invSchemaToMap(schema *invjsonschema.Schema) map[string]any {
	if schema == nil {
		return nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{}
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

type easypConfigSchemaRoot struct {
	Version  string                     `json:"version,omitempty"`
	Lint     *easypConfigSchemaLint     `json:"lint,omitempty"`
	Deps     []string                   `json:"deps,omitempty"`
	Generate *easypConfigSchemaGenerate `json:"generate,omitempty"`
	Breaking *easypConfigSchemaBreaking `json:"breaking,omitempty"`
}

type easypConfigSchemaLint struct {
	Use                 []string            `json:"use,omitempty"`
	EnumZeroValueSuffix string              `json:"enum_zero_value_suffix,omitempty"`
	ServiceSuffix       string              `json:"service_suffix,omitempty"`
	Ignore              []string            `json:"ignore,omitempty"`
	Except              []string            `json:"except,omitempty"`
	AllowCommentIgnores bool                `json:"allow_comment_ignores,omitempty"`
	IgnoreOnly          map[string][]string `json:"ignore_only,omitempty"`
}

type easypConfigSchemaGenerate struct {
	Inputs  []easypConfigSchemaInput  `json:"inputs"`
	Plugins []easypConfigSchemaPlugin `json:"plugins"`
	Managed *easypConfigSchemaManaged `json:"managed,omitempty"`
}

func (easypConfigSchemaGenerate) JSONSchemaExtend(schema *invjsonschema.Schema) {
	setMinItems(schema, "inputs", 1)
	setMinItems(schema, "plugins", 1)
}

type easypConfigSchemaInput struct {
	Directory easypConfigSchemaInputDirectory `json:"directory,omitempty"`
	GitRepo   *easypConfigSchemaInputGitRepo  `json:"git_repo,omitempty"`
}

func (easypConfigSchemaInput) JSONSchemaExtend(schema *invjsonschema.Schema) {
	schema.OneOf = []*invjsonschema.Schema{
		{Required: []string{"directory"}},
		{Required: []string{"git_repo"}},
	}
}

type easypConfigSchemaInputDirectory struct{}

func (easypConfigSchemaInputDirectory) JSONSchema() *invjsonschema.Schema {
	reflector := &invjsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
	}
	objectSchema := reflector.Reflect(easypConfigSchemaInputDirectoryObject{})
	objectSchema.Version = ""
	objectSchema.ID = ""
	objectSchema.Definitions = nil
	objectSchema.Title = ""

	return &invjsonschema.Schema{
		OneOf: []*invjsonschema.Schema{
			{Type: "string"},
			objectSchema,
		},
	}
}

type easypConfigSchemaInputDirectoryObject struct {
	Path string `json:"path"`
	Root string `json:"root,omitempty"`
}

type easypConfigSchemaInputGitRepo struct {
	URL          string `json:"url"`
	SubDirectory string `json:"sub_directory,omitempty"`
	Root         string `json:"root,omitempty"`
}

type easypConfigSchemaPlugin struct {
	Name        string                     `json:"name,omitempty"`
	Remote      string                     `json:"remote,omitempty"`
	Path        string                     `json:"path,omitempty"`
	Command     []string                   `json:"command,omitempty"`
	Out         string                     `json:"out"`
	Opts        easypConfigSchemaPluginOps `json:"opts,omitempty"`
	WithImports bool                       `json:"with_imports,omitempty"`
}

func (easypConfigSchemaPlugin) JSONSchemaExtend(schema *invjsonschema.Schema) {
	schema.OneOf = []*invjsonschema.Schema{
		{Required: []string{"name"}},
		{Required: []string{"remote"}},
		{Required: []string{"path"}},
		{Required: []string{"command"}},
	}
}

type easypConfigSchemaPluginOps map[string]any

func (easypConfigSchemaPluginOps) JSONSchema() *invjsonschema.Schema {
	return &invjsonschema.Schema{
		Type: "object",
		AdditionalProperties: &invjsonschema.Schema{
			OneOf: []*invjsonschema.Schema{
				{Type: "string"},
				{
					Type:  "array",
					Items: &invjsonschema.Schema{Type: "string"},
				},
			},
		},
	}
}

type easypConfigSchemaManaged struct {
	Enabled  bool                                   `json:"enabled,omitempty"`
	Disable  []easypConfigSchemaManagedDisableRule  `json:"disable,omitempty"`
	Override []easypConfigSchemaManagedOverrideRule `json:"override,omitempty"`
}

type easypConfigSchemaManagedDisableRule struct {
	Module      string `json:"module,omitempty"`
	Path        string `json:"path,omitempty"`
	FileOption  string `json:"file_option,omitempty"`
	FieldOption string `json:"field_option,omitempty"`
	Field       string `json:"field,omitempty"`
}

func (easypConfigSchemaManagedDisableRule) JSONSchemaExtend(schema *invjsonschema.Schema) {
	schema.AnyOf = []*invjsonschema.Schema{
		{Required: []string{"module"}},
		{Required: []string{"path"}},
		{Required: []string{"file_option"}},
		{Required: []string{"field_option"}},
		{Required: []string{"field"}},
	}
	schema.Not = &invjsonschema.Schema{Required: []string{"file_option", "field_option"}}
	schema.DependentRequired = map[string][]string{
		"field": {"field_option"},
	}
}

type easypConfigSchemaManagedOverrideRule struct {
	FileOption  string `json:"file_option,omitempty"`
	FieldOption string `json:"field_option,omitempty"`
	Value       any    `json:"value"`
	Module      string `json:"module,omitempty"`
	Path        string `json:"path,omitempty"`
	Field       string `json:"field,omitempty"`
}

func (easypConfigSchemaManagedOverrideRule) JSONSchemaExtend(schema *invjsonschema.Schema) {
	schema.AnyOf = []*invjsonschema.Schema{
		{Required: []string{"file_option"}},
		{Required: []string{"field_option"}},
	}
	schema.Not = &invjsonschema.Schema{Required: []string{"file_option", "field_option"}}
	schema.DependentRequired = map[string][]string{
		"field": {"field_option"},
	}
}

type easypConfigSchemaBreaking struct {
	Ignore        []string `json:"ignore,omitempty"`
	AgainstGitRef string   `json:"against_git_ref,omitempty"`
}

func setMinItems(schema *invjsonschema.Schema, fieldName string, min uint64) {
	if schema == nil || schema.Properties == nil {
		return
	}

	itemSchema, ok := schema.Properties.Get(fieldName)
	if !ok || itemSchema == nil {
		return
	}

	itemSchema.MinItems = &min
}
