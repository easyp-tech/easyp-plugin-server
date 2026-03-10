package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeature_String(t *testing.T) {
	tests := []struct {
		feature Feature
		want    string
	}{
		{FeatureCodeGeneration, "code_generation"},
		{FeaturePluginListing, "plugin_listing"},
		{FeatureMCPServerTools, "mcp_server_tools"},
		{FeatureRateLimiting, "rate_limiting"},
		{FeaturePluginCRUD, "plugin_crud"},
		{FeatureMultiTenancy, "multi_tenancy"},
		{FeatureResponseCaching, "response_caching"},
		{FeatureAudit, "audit"},
		{Feature(-1), "unknown"},
		{featureCount, "unknown"},
		{Feature(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.feature.String())
		})
	}
}

func TestFeature_IsEnterprise(t *testing.T) {
	enterpriseFeatures := []Feature{FeatureMultiTenancy, FeatureResponseCaching, FeatureAudit}
	communityFeatures := []Feature{
		FeatureCodeGeneration, FeaturePluginListing, FeatureMCPServerTools,
		FeatureRateLimiting, FeaturePluginCRUD,
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
	for f := FeatureCodeGeneration; f < featureCount; f++ {
		assert.True(t, f.Valid(), "%s should be valid", f)
	}

	// Out-of-range values should be invalid.
	assert.False(t, Feature(-1).Valid())
	assert.False(t, featureCount.Valid())
	assert.False(t, Feature(999).Valid())
}
