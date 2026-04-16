package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/lib/pq"

	"github.com/easyp-tech/service/internal/core"
)

// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6**
// Property 2: Preservation — Business logic of Get/List/Health/Close is unchanged
//
// These tests observe and capture the baseline behavior of the UNFIXED code.
// They MUST PASS on the current code to confirm the behavior we want to preserve.

// preservationSourceText reads registry.go and returns its content as a string.
func preservationSourceText(t *testing.T) string {
	t.Helper()
	path := registrySourcePath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read registry.go: %v", err)
	}

	return string(data)
}

// preservationNodeSource extracts source text for an AST node from registry.go.
func preservationNodeSource(t *testing.T, node ast.Node) string {
	t.Helper()
	src := preservationSourceText(t)
	start := int(node.Pos()) - 1
	end := int(node.End()) - 1
	if start < 0 || end > len(src) {
		t.Fatal("AST node position out of range")
	}

	return src[start:end]
}

// preservationFindMethod locates a method declaration by name in the AST.
func preservationFindMethod(t *testing.T, f *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("method %q not found in registry.go", name)

	return nil
}

// --- Requirement 3.1: Get with correct group/name/version returns plugin with parsed config ---

// TestPreservation_GetQueryStructure verifies that Get constructs the correct SQL
// query for a specific version lookup: selecting all required columns with 3 positional args.
func TestPreservation_GetQueryStructure(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Get")
	src := preservationNodeSource(t, fn.Body)

	expectedCols := []string{"id", "group_name", "name", "version", "config", "tags", "created_at"}
	for _, col := range expectedCols {
		if !strings.Contains(src, col) {
			t.Errorf("Get query must select column %q", col)
		}
	}

	for _, param := range []string{"$1", "$2", "$3"} {
		if !strings.Contains(src, param) {
			t.Errorf("Get query must use positional parameter %s", param)
		}
	}
}

// TestPreservation_GetParsesPluginConfig verifies that Get parses JSON config
// from the database into the pluginConfig field via json.Unmarshal.
func TestPreservation_GetParsesPluginConfig(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Get")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "json.Unmarshal") {
		t.Fatal("Get must call json.Unmarshal to parse plugin config from DB")
	}
	if !strings.Contains(src, "pluginConfig") {
		t.Fatal("Get must populate pluginConfig field after unmarshalling")
	}
}

// TestPreservation_GetSetsPluginDomain verifies that Get assigns the registry's
// domain to the returned plugin, enabling correct Docker image name construction.
func TestPreservation_GetSetsPluginDomain(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Get")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "dbFormat.domain") || !strings.Contains(src, "r.domain") {
		t.Fatal("Get must assign r.domain to the returned plugin's domain field")
	}
}

// TestPreservation_PluginInfoMapping verifies that plugin.Info() correctly maps
// all DB fields to core.PluginInfo using property-based testing with random data.
func TestPreservation_PluginInfoMapping(t *testing.T) {
	prop := func(group, name, version string, tagCount uint8) bool {
		count := int(tagCount % 10)
		tags := make(pq.StringArray, count)
		for i := range tags {
			tags[i] = fmt.Sprintf("tag-%d", i)
		}

		id := uuid.Must(uuid.NewV4())
		now := time.Now().Truncate(time.Microsecond)

		p := &plugin{
			ID:        id,
			GroupName: group,
			Name:      name,
			Version:   version,
			Tags:      tags,
			CreatedAt: now,
		}

		info := p.Info(context.Background())

		return info.ID == id &&
			info.Group == group &&
			info.Name == name &&
			info.Version == version &&
			info.CreatedAt.Equal(now) &&
			len(info.Tags) == count
	}

	err := quick.Check(prop, &quick.Config{MaxCount: 200})
	if err != nil {
		t.Fatalf("PluginInfo mapping property violated: %v", err)
	}
}

// --- Requirement 3.2: Get with version="latest" returns latest version ---

// TestPreservation_GetLatestVersionQuery verifies that when version is "latest",
// Get uses ORDER BY version DESC LIMIT 1 with only group and name parameters.
func TestPreservation_GetLatestVersionQuery(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Get")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, `"latest"`) {
		t.Fatal("Get must handle version='latest' as a special case")
	}
	if !strings.Contains(strings.ToLower(src), "order by version desc limit 1") {
		t.Fatal("Get must use 'ORDER BY version DESC LIMIT 1' for latest version lookup")
	}

	latestIdx := strings.Index(src, `"latest"`)
	if latestIdx == -1 {
		t.Fatal("cannot find latest branch")
	}
	latestSection := src[latestIdx:]
	if !strings.Contains(latestSection, "$1") || !strings.Contains(latestSection, "$2") {
		t.Fatal("latest query must use $1 (group) and $2 (name)")
	}
}

// --- Requirement 3.3: Get with non-existent plugin returns core.ErrNotFound ---

// TestPreservation_GetReturnsErrNotFound verifies that Get wraps sql.ErrNoRows
// into core.ErrNotFound with the plugin identifier in the error message.
func TestPreservation_GetReturnsErrNotFound(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Get")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "sql.ErrNoRows") {
		t.Fatal("Get must check for sql.ErrNoRows to detect missing plugins")
	}
	if !strings.Contains(src, "ErrNotFound") {
		t.Fatal("Get must return core.ErrNotFound when plugin is not found")
	}
	if !strings.Contains(src, "pluginGroup") || !strings.Contains(src, "pluginName") || !strings.Contains(src, "pluginVersion") {
		t.Fatal("Get must include group/name/version in the ErrNotFound error message")
	}
}

// --- Requirement 3.4: List with filters returns filtered list ---

// TestPreservation_ListFilterQueryConstruction verifies that List dynamically
// builds SQL query with filters for group, name, version, and tags.
func TestPreservation_ListFilterQueryConstruction(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "List")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "select") || !strings.Contains(src, "plugins") {
		t.Fatal("List must query the plugins table")
	}
	if !strings.Contains(src, "filter.Group") || !strings.Contains(src, "group_name") {
		t.Fatal("List must filter by group_name when filter.Group is set")
	}
	if !strings.Contains(src, "filter.Name") {
		t.Fatal("List must filter by name when filter.Name is set")
	}
	if !strings.Contains(src, "filter.Version") {
		t.Fatal("List must filter by version when filter.Version is set")
	}
	if !strings.Contains(src, "filter.Tags") || !strings.Contains(src, "@>") {
		t.Fatal("List must filter by tags using @> (array containment) operator")
	}
	if !strings.Contains(src, "argID") {
		t.Fatal("List must use dynamic argID for positional parameter numbering")
	}
}

// TestPreservation_ListTagsFilterSkipsEmpty verifies that List filters out
// empty strings from tags before applying the filter.
func TestPreservation_ListTagsFilterSkipsEmpty(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "List")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "nonEmptyTags") {
		t.Fatal("List must filter out empty strings from tags before applying filter")
	}
}

// TestPreservation_ListReturnsPluginInfoSlice verifies that List converts
// internal plugin structs to core.PluginInfo slice via Info() method.
func TestPreservation_ListReturnsPluginInfoSlice(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "List")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, ".Info(") {
		t.Fatal("List must call Info() to convert plugins to core.PluginInfo")
	}
	if !strings.Contains(src, "core.PluginInfo") {
		t.Fatal("List must return []core.PluginInfo")
	}
}

// TestPreservation_ListUsesSelectContext verifies that List uses SelectContext
// for multi-row queries (not GetContext which is for single rows).
func TestPreservation_ListUsesSelectContext(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "List")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "SelectContext") {
		t.Fatal("List must use SelectContext for multi-row queries")
	}
}

// --- Requirement 3.5: Health checks DB availability ---

// TestPreservation_HealthUsesPing verifies that Health checks DB availability
// by calling PingContext on the database connection.
func TestPreservation_HealthUsesPing(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Health")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, "PingContext") {
		t.Fatal("Health must use PingContext to check DB availability")
	}
}

// TestPreservation_HealthAcceptsContext verifies that Health method accepts
// a context.Context parameter for cancellation/timeout support.
func TestPreservation_HealthAcceptsContext(t *testing.T) {
	rt := reflect.TypeFor[*Registry]()
	method, ok := rt.MethodByName("Health")
	if !ok {
		t.Fatal("Registry must have a Health method")
	}

	mt := method.Type
	if mt.NumIn() != 2 {
		t.Fatalf("Health must accept exactly 1 parameter (context.Context), got %d params", mt.NumIn()-1)
	}

	ctxType := reflect.TypeFor[context.Context]()
	if !mt.In(1).Implements(ctxType) {
		t.Fatalf("Health parameter must be context.Context, got %s", mt.In(1))
	}

	if mt.NumOut() != 1 {
		t.Fatalf("Health must return 1 value (error), returns %d", mt.NumOut())
	}
	errType := reflect.TypeFor[error]()
	if !mt.Out(0).Implements(errType) {
		t.Fatalf("Health must return error, got %s", mt.Out(0))
	}
}

// --- Requirement 3.6: Close correctly closes the connection ---

// TestPreservation_CloseCallsDBClose verifies that Close delegates to the
// underlying database connection's Close method.
func TestPreservation_CloseCallsDBClose(t *testing.T) {
	f := parseRegistryAST(t)
	fn := preservationFindMethod(t, f, "Close")
	src := preservationNodeSource(t, fn.Body)

	if !strings.Contains(src, ".Close()") {
		t.Fatal("Close must delegate to the underlying DB connection's Close()")
	}
}

// TestPreservation_CloseReturnsError verifies that Close returns an error
// to allow callers to handle connection cleanup failures.
func TestPreservation_CloseReturnsError(t *testing.T) {
	rt := reflect.TypeFor[*Registry]()
	method, ok := rt.MethodByName("Close")
	if !ok {
		t.Fatal("Registry must have a Close method")
	}

	mt := method.Type
	if mt.NumIn() != 1 {
		t.Fatalf("Close must accept no parameters, got %d", mt.NumIn()-1)
	}
	if mt.NumOut() != 1 {
		t.Fatalf("Close must return 1 value (error), returns %d", mt.NumOut())
	}
	errType := reflect.TypeFor[error]()
	if !mt.Out(0).Implements(errType) {
		t.Fatalf("Close must return error, got %s", mt.Out(0))
	}
}

// --- Property-based tests for pure business logic ---

// TestPreservation_PluginConfigParsing verifies that JSON config parsing
// preserves Docker configuration fields using property-based testing.
func TestPreservation_PluginConfigParsing(t *testing.T) {
	prop := func(network, memory, cpus, user string, readOnly bool) bool {
		dc := DockerConfig{
			Network:  network,
			Memory:   memory,
			CPUs:     cpus,
			User:     user,
			ReadOnly: readOnly,
		}

		pc := PluginConfig{Docker: &dc}
		data, err := json.Marshal(pc)
		if err != nil {
			return false
		}

		var parsed PluginConfig
		if err := json.Unmarshal(data, &parsed); err != nil {
			return false
		}

		if parsed.Docker == nil {
			return false
		}

		return parsed.Docker.Network == network &&
			parsed.Docker.Memory == memory &&
			parsed.Docker.CPUs == cpus &&
			parsed.Docker.User == user &&
			parsed.Docker.ReadOnly == readOnly
	}

	err := quick.Check(prop, &quick.Config{MaxCount: 200})
	if err != nil {
		t.Fatalf("PluginConfig parsing property violated: %v", err)
	}
}

// --- Interface compliance preservation ---

// TestPreservation_RegistryImplementsCoreRegistry verifies that Registry
// implements the core.Registry interface (Get + List methods).
func TestPreservation_RegistryImplementsCoreRegistry(t *testing.T) {
	registryType := reflect.TypeFor[*Registry]()
	ifaceType := reflect.TypeFor[core.Registry]()

	if !registryType.Implements(ifaceType) {
		t.Fatal("Registry must implement core.Registry interface")
	}
}

// TestPreservation_PluginImplementsCorePlugin verifies that plugin
// implements the core.Plugin interface (Generate + Info methods).
func TestPreservation_PluginImplementsCorePlugin(t *testing.T) {
	pluginType := reflect.TypeFor[*plugin]()
	ifaceType := reflect.TypeFor[core.Plugin]()

	if !pluginType.Implements(ifaceType) {
		t.Fatal("plugin must implement core.Plugin interface")
	}
}

// TestPreservation_GetMethodSignature verifies the Get method signature
// matches the core.Registry interface exactly.
func TestPreservation_GetMethodSignature(t *testing.T) {
	rt := reflect.TypeFor[*Registry]()
	method, ok := rt.MethodByName("Get")
	if !ok {
		t.Fatal("Registry must have a Get method")
	}

	mt := method.Type
	if mt.NumIn() != 5 {
		t.Fatalf("Get must accept 4 parameters, got %d", mt.NumIn()-1)
	}
	if mt.NumOut() != 2 {
		t.Fatalf("Get must return 2 values, returns %d", mt.NumOut())
	}

	pluginType := reflect.TypeFor[core.Plugin]()
	if !mt.Out(0).Implements(pluginType) {
		t.Fatalf("Get first return must implement core.Plugin, got %s", mt.Out(0))
	}
	errType := reflect.TypeFor[error]()
	if !mt.Out(1).Implements(errType) {
		t.Fatalf("Get second return must be error, got %s", mt.Out(1))
	}
}

// TestPreservation_ListMethodSignature verifies the List method signature
// matches the core.Registry interface exactly.
func TestPreservation_ListMethodSignature(t *testing.T) {
	rt := reflect.TypeFor[*Registry]()
	method, ok := rt.MethodByName("List")
	if !ok {
		t.Fatal("Registry must have a List method")
	}

	mt := method.Type
	if mt.NumIn() != 3 {
		t.Fatalf("List must accept 2 parameters, got %d", mt.NumIn()-1)
	}
	if mt.NumOut() != 2 {
		t.Fatalf("List must return 2 values, returns %d", mt.NumOut())
	}
}
