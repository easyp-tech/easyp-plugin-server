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
