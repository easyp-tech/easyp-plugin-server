package mcpserver

import "github.com/google/jsonschema-go/jsonschema"

func pluginsListInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"group": {
				Type:        "string",
				Description: "Filter by exact plugin group (optional)",
			},
			"name": {
				Type:        "string",
				Description: "Filter by exact plugin name (optional)",
			},
			"version": {
				Type:        "string",
				Description: "Filter by exact plugin version (optional)",
			},
			"tags": {
				Type:        "array",
				Description: "Filter by tags; plugin must contain all specified tags (optional)",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
	}
}

func pluginsListOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"total": {
				Type:        "integer",
				Description: "Number of plugins in this response",
			},
			"plugins": {
				Type:        "array",
				Description: "Matching plugins",
				Items: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"id": {
							Type:        "string",
							Description: "Plugin UUID",
						},
						"group": {
							Type:        "string",
							Description: "Plugin group",
						},
						"name": {
							Type:        "string",
							Description: "Plugin name",
						},
						"version": {
							Type:        "string",
							Description: "Plugin version",
						},
						"tags": {
							Type:        "array",
							Description: "Plugin tags",
							Items: &jsonschema.Schema{
								Type: "string",
							},
						},
						"created_at": {
							Type:        "string",
							Description: "Creation timestamp in RFC3339 format",
						},
					},
					Required: []string{"id", "group", "name", "version", "created_at"},
				},
			},
		},
		Required: []string{"total", "plugins"},
	}
}

func easypConfigDescribeInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"path": {
				Type:        "string",
				Description: "Dot path to a section of the schema. Empty means full schema",
			},
			"include_schema": {
				Type:        "boolean",
				Description: "Include JSON schema fragment in output. Default: true",
			},
			"include_fields": {
				Type:        "boolean",
				Description: "Include field documentation in output. Default: true",
			},
			"include_examples": {
				Type:        "boolean",
				Description: "Include examples in output. Default: true",
			},
			"include_children": {
				Type:        "boolean",
				Description: "Include descendants of selected path. Default: true",
			},
			"examples_limit": {
				Type:        "integer",
				Description: "Maximum number of examples to return. Default: 10, range 1..50",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(50.0),
			},
		},
	}
}

func easypConfigDescribeOutputSchema() *jsonschema.Schema {
	fieldDocSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"path": {
				Type:        "string",
				Description: "Field path",
			},
			"type": {
				Type:        "string",
				Description: "Value type",
			},
			"required": {
				Type:        "boolean",
				Description: "Whether field is required",
			},
			"description": {
				Type:        "string",
				Description: "Field purpose",
			},
			"allowed_values": {
				Type:        "array",
				Description: "Allowed values or enum options",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"default_value": {
				Type:        "string",
				Description: "Default value if omitted",
			},
			"examples": {
				Type:        "array",
				Description: "Value examples",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"notes": {
				Type:        "array",
				Description: "Extra constraints or caveats",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"path", "type", "required", "description"},
	}

	exampleSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"title": {
				Type:        "string",
				Description: "Short example title",
			},
			"description": {
				Type:        "string",
				Description: "Example purpose",
			},
			"yaml": {
				Type:        "string",
				Description: "YAML snippet",
			},
			"paths": {
				Type:        "array",
				Description: "Schema paths covered by this example",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"title", "yaml"},
	}

	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"schema_version": {
				Type:        "string",
				Description: "Schema metadata version",
			},
			"selected_path": {
				Type:        "string",
				Description: "Resolved path used for this response",
			},
			"schema": {
				Type:        "object",
				Description: "JSON schema fragment for selected path",
			},
			"fields": {
				Type:        "array",
				Description: "Field documentation",
				Items:       fieldDocSchema,
			},
			"examples": {
				Type:        "array",
				Description: "YAML examples",
				Items:       exampleSchema,
			},
			"notes": {
				Type:        "array",
				Description: "General notes and caveats",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"schema_version", "selected_path"},
	}
}
