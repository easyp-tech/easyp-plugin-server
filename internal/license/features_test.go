package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeature_String(t *testing.T) {
	tests := []struct {
		f    feature
		want string
	}{
		{featureCodeGeneration, "code_generation"},
		{featurePluginListing, "plugin_listing"},
		{featureMCPServerTools, "mcp_server_tools"},
		{featureRateLimiting, "rate_limiting"},
		{featurePluginCRUD, "plugin_crud"},
		{featureMultiTenancy, "multi_tenancy"},
		{featureResponseCaching, "response_caching"},
		{featureAudit, "audit"},
		{feature(-1), "unknown"},
		{featureCount, "unknown"},
		{feature(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.f.String())
		})
	}
}

func TestFeature_IsEnterprise(t *testing.T) {
	enterpriseFeatures := []feature{featureMultiTenancy, featureResponseCaching, featureAudit}
	communityFeatures := []feature{
		featureCodeGeneration, featurePluginListing, featureMCPServerTools,
		featureRateLimiting, featurePluginCRUD,
	}

	for _, f := range enterpriseFeatures {
		assert.True(t, f.IsEnterprise(), "%s should be enterprise", f)
	}
	for _, f := range communityFeatures {
		assert.False(t, f.IsEnterprise(), "%s should not be enterprise", f)
	}
}

func TestFeature_Valid(t *testing.T) {
	// All defined features should be valid.
	for f := range featureCount {
		assert.True(t, f.Valid(), "%s should be valid", f)
	}

	// Out-of-range values should be invalid.
	assert.False(t, feature(-1).Valid())
	assert.False(t, featureCount.Valid())
	assert.False(t, feature(999).Valid())
}
