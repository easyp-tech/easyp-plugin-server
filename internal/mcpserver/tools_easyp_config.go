package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	lintRuleGroups = []string{
		"MINIMAL",
		"BASIC",
		"DEFAULT",
		"COMMENTS",
		"UNARY_RPC",
	}
	lintRuleNames = []string{
		"DIRECTORY_SAME_PACKAGE",
		"PACKAGE_DEFINED",
		"PACKAGE_DIRECTORY_MATCH",
		"PACKAGE_SAME_DIRECTORY",
		"ENUM_FIRST_VALUE_ZERO",
		"ENUM_NO_ALLOW_ALIAS",
		"ENUM_PASCAL_CASE",
		"ENUM_VALUE_UPPER_SNAKE_CASE",
		"FIELD_LOWER_SNAKE_CASE",
		"IMPORT_NO_PUBLIC",
		"IMPORT_NO_WEAK",
		"IMPORT_USED",
		"MESSAGE_PASCAL_CASE",
		"ONEOF_LOWER_SNAKE_CASE",
		"PACKAGE_LOWER_SNAKE_CASE",
		"PACKAGE_SAME_CSHARP_NAMESPACE",
		"PACKAGE_SAME_GO_PACKAGE",
		"PACKAGE_SAME_JAVA_MULTIPLE_FILES",
		"PACKAGE_SAME_JAVA_PACKAGE",
		"PACKAGE_SAME_PHP_NAMESPACE",
		"PACKAGE_SAME_RUBY_PACKAGE",
		"PACKAGE_SAME_SWIFT_PREFIX",
		"RPC_PASCAL_CASE",
		"SERVICE_PASCAL_CASE",
		"ENUM_VALUE_PREFIX",
		"ENUM_ZERO_VALUE_SUFFIX",
		"FILE_LOWER_SNAKE_CASE",
		"RPC_REQUEST_RESPONSE_UNIQUE",
		"RPC_REQUEST_STANDARD_NAME",
		"RPC_RESPONSE_STANDARD_NAME",
		"PACKAGE_VERSION_SUFFIX",
		"SERVICE_SUFFIX",
		"COMMENT_ENUM",
		"COMMENT_ENUM_VALUE",
		"COMMENT_FIELD",
		"COMMENT_MESSAGE",
		"COMMENT_ONEOF",
		"COMMENT_RPC",
		"COMMENT_SERVICE",
		"RPC_NO_CLIENT_STREAMING",
		"RPC_NO_SERVER_STREAMING",
		"PACKAGE_NO_IMPORT_CYCLE",
	}
)

type easypConfigDescribeInput struct {
	Path            string `json:"path,omitempty"`
	IncludeSchema   *bool  `json:"include_schema,omitempty"`
	IncludeFields   *bool  `json:"include_fields,omitempty"`
	IncludeExamples *bool  `json:"include_examples,omitempty"`
	IncludeChildren *bool  `json:"include_children,omitempty"`
	ExamplesLimit   *int   `json:"examples_limit,omitempty"`
}

type easypFieldDoc struct {
	Path          string   `json:"path"`
	Type          string   `json:"type"`
	Required      bool     `json:"required"`
	Description   string   `json:"description"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	DefaultValue  string   `json:"default_value,omitempty"`
	Examples      []string `json:"examples,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

type easypExample struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	YAML        string   `json:"yaml"`
	Paths       []string `json:"paths,omitempty"`
}

type easypConfigDescribeOutput struct {
	SchemaVersion string          `json:"schema_version"`
	SelectedPath  string          `json:"selected_path"`
	Schema        map[string]any  `json:"schema,omitempty"`
	Fields        []easypFieldDoc `json:"fields,omitempty"`
	Examples      []easypExample  `json:"examples,omitempty"`
	Notes         []string        `json:"notes,omitempty"`
}

type easypNodeDoc struct {
	Fields   []easypFieldDoc
	Examples []easypExample
	Notes    []string
}

type easypSpec struct {
	SchemaVersion string
	SchemaByPath  map[string]map[string]any
	DocsByPath    map[string]easypNodeDoc
}

func registerEasypConfigTools(server *mcp.Server) {
	spec := newEasypSpec()

	mcp.AddTool(server, &mcp.Tool{
		Name:         "easyp.config.describe",
		Description:  "Describe easyp.yaml schema and field usage. Supports full schema or a specific path with examples.",
		InputSchema:  easypConfigDescribeInputSchema(),
		OutputSchema: easypConfigDescribeOutputSchema(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input easypConfigDescribeInput) (*mcp.CallToolResult, easypConfigDescribeOutput, error) {
		out, err := spec.describe(input)
		if err != nil {
			return nil, easypConfigDescribeOutput{}, err
		}
		return nil, out, nil
	})
}

func (s easypSpec) describe(input easypConfigDescribeInput) (easypConfigDescribeOutput, error) {
	selectedPath, ok := s.resolvePath(input.Path)
	if !ok {
		return easypConfigDescribeOutput{}, fmt.Errorf("unknown path %q", input.Path)
	}

	includeSchema := boolOrDefault(input.IncludeSchema, true)
	includeFields := boolOrDefault(input.IncludeFields, true)
	includeExamples := boolOrDefault(input.IncludeExamples, true)
	includeChildren := boolOrDefault(input.IncludeChildren, true)
	examplesLimit := intOrDefault(input.ExamplesLimit, 10)
	if examplesLimit < 1 {
		examplesLimit = 1
	}
	if examplesLimit > 50 {
		examplesLimit = 50
	}

	paths := s.pathsFor(selectedPath, includeChildren)

	out := easypConfigDescribeOutput{
		SchemaVersion: s.SchemaVersion,
		SelectedPath:  selectedPath,
	}

	if includeSchema {
		out.Schema = s.SchemaByPath[selectedPath]
	}
	if includeFields {
		out.Fields = s.collectFields(paths)
	}
	if includeExamples {
		out.Examples = s.collectExamples(paths, examplesLimit)
	}
	out.Notes = s.collectNotes(paths)

	return out, nil
}

func (s easypSpec) resolvePath(rawPath string) (string, bool) {
	path := normalizePath(rawPath)
	if s.hasPath(path) {
		return path, true
	}

	normPath := removeArrayMarkers(path)
	for _, candidate := range s.allPaths() {
		if removeArrayMarkers(candidate) == normPath {
			return candidate, true
		}
	}
	return "", false
}

func (s easypSpec) pathsFor(selectedPath string, includeChildren bool) []string {
	if !includeChildren {
		return []string{selectedPath}
	}

	allPaths := s.allPaths()
	paths := make([]string, 0, len(allPaths))
	for _, p := range allPaths {
		if isPathWithin(selectedPath, p) {
			paths = append(paths, p)
		}
	}
	return paths
}

func (s easypSpec) collectFields(paths []string) []easypFieldDoc {
	seen := make(map[string]struct{})
	out := make([]easypFieldDoc, 0)
	for _, p := range paths {
		doc, ok := s.DocsByPath[p]
		if !ok {
			continue
		}
		for _, f := range doc.Fields {
			if _, exists := seen[f.Path]; exists {
				continue
			}
			seen[f.Path] = struct{}{}
			out = append(out, f)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func (s easypSpec) collectExamples(paths []string, limit int) []easypExample {
	out := make([]easypExample, 0, limit)
	seen := make(map[string]struct{})
	for _, p := range paths {
		doc, ok := s.DocsByPath[p]
		if !ok {
			continue
		}
		for _, ex := range doc.Examples {
			if _, exists := seen[ex.Title]; exists {
				continue
			}
			seen[ex.Title] = struct{}{}
			out = append(out, ex)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func (s easypSpec) collectNotes(paths []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, p := range paths {
		doc, ok := s.DocsByPath[p]
		if !ok {
			continue
		}
		for _, note := range doc.Notes {
			if _, exists := seen[note]; exists {
				continue
			}
			seen[note] = struct{}{}
			out = append(out, note)
		}
	}
	return out
}

func (s easypSpec) hasPath(path string) bool {
	if _, ok := s.SchemaByPath[path]; ok {
		return true
	}
	if _, ok := s.DocsByPath[path]; ok {
		return true
	}
	return false
}

func (s easypSpec) allPaths() []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, len(s.SchemaByPath)+len(s.DocsByPath))

	for p := range s.SchemaByPath {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for p := range s.DocsByPath {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}

	sort.Strings(paths)
	return paths
}

func boolOrDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func intOrDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" || strings.EqualFold(path, "root") {
		return "$"
	}

	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, ".")
	path = strings.ReplaceAll(path, "[*]", "[]")
	path = strings.ReplaceAll(path, "[0]", "[]")
	path = strings.TrimSuffix(path, ".")

	return path
}

func removeArrayMarkers(path string) string {
	return strings.ReplaceAll(path, "[]", "")
}

func isPathWithin(base, candidate string) bool {
	if base == "$" {
		return true
	}
	if base == candidate {
		return true
	}
	if strings.HasPrefix(candidate, base+".") {
		return true
	}
	if strings.HasPrefix(candidate, base+"[].") {
		return true
	}
	if candidate == base+"[]" {
		return true
	}
	return false
}

func newEasypSpec() easypSpec {
	docs := map[string]easypNodeDoc{
		"$": {
			Fields: []easypFieldDoc{
				{Path: "version", Type: "string", Required: false, Description: "Legacy compatibility field.", DefaultValue: "omitted", Examples: []string{"v1alpha"}},
				{Path: "lint", Type: "object", Required: false, Description: "Linter configuration and rule selection."},
				{Path: "deps", Type: "array<string>", Required: false, Description: "Dependency repositories in format <repo>@<version>."},
				{Path: "generate", Type: "object", Required: false, Description: "Code generation configuration."},
				{Path: "breaking", Type: "object", Required: false, Description: "Breaking changes check configuration."},
			},
			Examples: []easypExample{
				{
					Title:       "minimal_config",
					Description: "Small valid configuration with local input and one plugin.",
					YAML:        "lint:\n  use:\n    - DIRECTORY_SAME_PACKAGE\ngenerate:\n  inputs:\n    - directory: proto\n  plugins:\n    - name: go\n      out: .\n      opts:\n        paths: source_relative\n",
					Paths:       []string{"$", "lint", "generate"},
				},
			},
			Notes: []string{
				"`generate.inputs[].git_repo.out` is intentionally excluded: it is treated as invalid and must not be used.",
				"`generate.plugins[].url` is not a valid field in current schema; use `generate.plugins[].remote`.",
				"Exactly one plugin source must be set per plugin item: name, remote, path, or command.",
			},
		},
		"lint": {
			Fields: []easypFieldDoc{
				{Path: "lint.use", Type: "array<string>", Required: false, Description: "Rule groups and/or individual lint rule names.", AllowedValues: append(append([]string{}, lintRuleGroups...), lintRuleNames...), DefaultValue: "[]"},
				{Path: "lint.enum_zero_value_suffix", Type: "string", Required: false, Description: "Required suffix for enum zero value.", DefaultValue: "UNSPECIFIED (runtime default)"},
				{Path: "lint.service_suffix", Type: "string", Required: false, Description: "Required suffix for service names.", DefaultValue: "Service (runtime default)"},
				{Path: "lint.ignore", Type: "array<string>", Required: false, Description: "Paths to exclude from linting.", DefaultValue: "[]"},
				{Path: "lint.except", Type: "array<string>", Required: false, Description: "Rules to disable globally.", DefaultValue: "[]"},
				{Path: "lint.allow_comment_ignores", Type: "boolean", Required: false, Description: "Allow inline ignore comments in proto files.", DefaultValue: "false"},
				{Path: "lint.ignore_only", Type: "map<string, array<string>>", Required: false, Description: "Disable specific rules only for selected paths.", DefaultValue: "{}"},
			},
		},
		"deps": {
			Fields: []easypFieldDoc{
				{Path: "deps[]", Type: "string", Required: false, Description: "Dependency in format <repo>@<version>.", Examples: []string{"github.com/googleapis/googleapis@v1.0.0", "github.com/bufbuild/protoc-gen-validate"}},
			},
		},
		"generate": {
			Fields: []easypFieldDoc{
				{Path: "generate.inputs", Type: "array<object>", Required: true, Description: "Input sources for proto files.", DefaultValue: "must be provided"},
				{Path: "generate.plugins", Type: "array<object>", Required: true, Description: "Plugin definitions for generation.", DefaultValue: "must be provided"},
				{Path: "generate.managed", Type: "object", Required: false, Description: "Managed mode rules for file/field options.", DefaultValue: "{}"},
			},
			Examples: []easypExample{
				{
					Title:       "generate_local_and_remote_plugin",
					Description: "Local directory input with remote plugin execution.",
					YAML:        "generate:\n  inputs:\n    - directory:\n        path: api\n        root: .\n  plugins:\n    - remote: api.easyp.tech/protobuf/go:v1.36.10\n      out: .\n      opts:\n        paths: source_relative\n",
					Paths:       []string{"generate", "generate.inputs", "generate.plugins"},
				},
			},
		},
		"generate.inputs": {
			Fields: []easypFieldDoc{
				{Path: "generate.inputs[].directory", Type: "string | object", Required: false, Description: "Local input directory. Shorthand string or object with path/root."},
				{Path: "generate.inputs[].git_repo", Type: "object", Required: false, Description: "Remote git repository input."},
			},
			Notes: []string{
				"Each input item must contain exactly one of `directory` or `git_repo`.",
			},
		},
		"generate.inputs[].directory": {
			Fields: []easypFieldDoc{
				{Path: "generate.inputs[].directory.path", Type: "string", Required: true, Description: "Directory with .proto files (relative to config root unless absolute).", Examples: []string{"proto", "api/proto"}},
				{Path: "generate.inputs[].directory.root", Type: "string", Required: false, Description: "Import root for path normalization.", DefaultValue: "."},
			},
		},
		"generate.inputs[].git_repo": {
			Fields: []easypFieldDoc{
				{Path: "generate.inputs[].git_repo.url", Type: "string", Required: true, Description: "Git repo URL with optional revision.", Examples: []string{"github.com/acme/common@v1.0.0"}},
				{Path: "generate.inputs[].git_repo.sub_directory", Type: "string", Required: false, Description: "Subdirectory inside checked-out repository."},
				{Path: "generate.inputs[].git_repo.root", Type: "string", Required: false, Description: "Import root under repository contents.", DefaultValue: "\"\""},
			},
			Notes: []string{
				"`generate.inputs[].git_repo.out` is not part of valid schema and must not be used.",
			},
		},
		"generate.plugins": {
			Fields: []easypFieldDoc{
				{Path: "generate.plugins[]", Type: "object", Required: true, Description: "Plugin item with exactly one source and required output directory."},
			},
		},
		"generate.plugins[]": {
			Fields: []easypFieldDoc{
				{Path: "generate.plugins[].name", Type: "string", Required: false, Description: "Built-in/local plugin name (one source option).", Examples: []string{"go", "go-grpc"}},
				{Path: "generate.plugins[].remote", Type: "string", Required: false, Description: "Remote plugin endpoint (one source option).", Examples: []string{"api.easyp.tech/protobuf/go:v1.36.10"}},
				{Path: "generate.plugins[].path", Type: "string", Required: false, Description: "Explicit path to plugin binary (one source option)."},
				{Path: "generate.plugins[].command", Type: "array<string>", Required: false, Description: "Command invocation for plugin (one source option).", Examples: []string{`["go","run","github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.25.1"]`}},
				{Path: "generate.plugins[].out", Type: "string", Required: true, Description: "Output directory for generated files.", Examples: []string{".", "gen/go"}},
				{Path: "generate.plugins[].opts", Type: "map<string, string | []string>", Required: false, Description: "Plugin options; value can be scalar or array of scalars."},
				{Path: "generate.plugins[].with_imports", Type: "boolean", Required: false, Description: "Include dependency protos in generation.", DefaultValue: "false"},
			},
			Notes: []string{
				"Use only one of `name`, `remote`, `path`, or `command` per item.",
				"`generate.plugins[].url` is invalid in current schema; use `remote`.",
			},
			Examples: []easypExample{
				{
					Title:       "plugin_remote",
					Description: "Remote plugin source.",
					YAML:        "generate:\n  plugins:\n    - remote: api.easyp.tech/grpc/go:v1.5.1\n      out: .\n      opts:\n        paths: source_relative\n",
					Paths:       []string{"generate.plugins[]"},
				},
				{
					Title:       "plugin_command",
					Description: "Command-based plugin source.",
					YAML:        "generate:\n  plugins:\n    - command: [\"go\", \"run\", \"github.com/bufbuild/protoc-gen-validate@v0.10.1\"]\n      out: gen/go\n",
					Paths:       []string{"generate.plugins[]"},
				},
			},
		},
		"generate.managed": {
			Fields: []easypFieldDoc{
				{Path: "generate.managed.enabled", Type: "boolean", Required: false, Description: "Enable managed mode option rewriting.", DefaultValue: "false"},
				{Path: "generate.managed.disable", Type: "array<object>", Required: false, Description: "Disable managed mode per module/path/option."},
				{Path: "generate.managed.override", Type: "array<object>", Required: false, Description: "Override file/field options with values."},
			},
		},
		"generate.managed.disable": {
			Fields: []easypFieldDoc{
				{Path: "generate.managed.disable[].module", Type: "string", Required: false, Description: "Apply disable to module."},
				{Path: "generate.managed.disable[].path", Type: "string", Required: false, Description: "Apply disable to path."},
				{Path: "generate.managed.disable[].file_option", Type: "string", Required: false, Description: "Disable this file option."},
				{Path: "generate.managed.disable[].field_option", Type: "string", Required: false, Description: "Disable this field option."},
				{Path: "generate.managed.disable[].field", Type: "string", Required: false, Description: "Field selector for field_option."},
			},
			Notes: []string{
				"At least one key in each disable item is required.",
				"`file_option` and `field_option` cannot be used together.",
				"`field` requires `field_option`.",
			},
		},
		"generate.managed.override": {
			Fields: []easypFieldDoc{
				{Path: "generate.managed.override[].file_option", Type: "string", Required: false, Description: "Target file option to override."},
				{Path: "generate.managed.override[].field_option", Type: "string", Required: false, Description: "Target field option to override."},
				{Path: "generate.managed.override[].value", Type: "any", Required: true, Description: "Override value."},
				{Path: "generate.managed.override[].module", Type: "string", Required: false, Description: "Optional module selector."},
				{Path: "generate.managed.override[].path", Type: "string", Required: false, Description: "Optional path selector."},
				{Path: "generate.managed.override[].field", Type: "string", Required: false, Description: "Optional field selector (for field_option)."},
			},
			Notes: []string{
				"Each override item requires exactly one of file_option or field_option.",
				"`field` can only be used with `field_option`.",
			},
		},
		"breaking": {
			Fields: []easypFieldDoc{
				{Path: "breaking.ignore", Type: "array<string>", Required: false, Description: "Paths excluded from breaking-change checks.", DefaultValue: "[]"},
				{Path: "breaking.against_git_ref", Type: "string", Required: false, Description: "Branch/tag/commit used for comparison."},
			},
		},
	}

	return easypSpec{
		SchemaVersion: "easyp-config-v1",
		SchemaByPath:  buildEasypConfigSchemaIndex(),
		DocsByPath:    docs,
	}
}
