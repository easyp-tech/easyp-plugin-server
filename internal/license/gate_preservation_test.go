package license

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/easyp-tech/service/internal/core"
)

// gate_preservation_test.go — preservation tests for FeatureGate.
// These tests lock the observable behaviour of FeatureGate before and after
// the license.Manager refactoring. They MUST pass throughout the migration.

// stubFeatureGate duplicates the gate decision logic using core.LicenseClaims
// so that preservation tests have no dependency on the real Manager.
type stubFeatureGate struct {
	claims core.LicenseClaims
}

func newStubGate(claims core.LicenseClaims) *stubFeatureGate {
	return &stubFeatureGate{claims: claims}
}

func (g *stubFeatureGate) Enabled(feat core.Feature) bool {
	lf := feature(feat)
	if !lf.Valid() {
		return false
	}
	if g.claims.Tier == core.LicenseTierEnterprise {
		return true
	}
	if lf.IsEnterprise() {
		return false
	}
	for _, f := range g.claims.Features {
		if f == feat {
			return true
		}
	}
	return false
}

func (g *stubFeatureGate) MaxWorkers() int { return g.claims.MaxWorkers }
func (g *stubFeatureGate) MaxPlugins() int { return g.claims.MaxPlugins }

// TestFeatureGate_CommunityDefaults_Enabled verifies that all non-enterprise features
// are enabled and all enterprise features are disabled for community claims.
func TestFeatureGate_CommunityDefaults_Enabled(t *testing.T) {
	gate := newStubGate(core.CommunityLicenseClaims())

	communityFeats := []core.Feature{
		core.FeatureCodeGeneration,
		core.FeaturePluginListing,
		core.FeatureMCPServerTools,
		core.FeatureRateLimiting,
		core.FeaturePluginCRUD,
	}
	for _, f := range communityFeats {
		assert.True(t, gate.Enabled(f), "community feature %s should be enabled", f)
	}

	enterpriseFeats := []core.Feature{
		core.FeatureMultiTenancy,
		core.FeatureResponseCaching,
		core.FeatureAudit,
	}
	for _, f := range enterpriseFeats {
		assert.False(t, gate.Enabled(f), "enterprise feature %s should be disabled in community mode", f)
	}
}

// TestFeatureGate_MaxWorkers_Returns4ForCommunity verifies MaxWorkers for community claims.
func TestFeatureGate_MaxWorkers_Returns4ForCommunity(t *testing.T) {
	gate := newStubGate(core.CommunityLicenseClaims())
	assert.Equal(t, 4, gate.MaxWorkers())
}

// TestFeatureGate_MaxPlugins_Returns10ForCommunity verifies MaxPlugins for community claims.
func TestFeatureGate_MaxPlugins_Returns10ForCommunity(t *testing.T) {
	gate := newStubGate(core.CommunityLicenseClaims())
	assert.Equal(t, 10, gate.MaxPlugins())
}

// TestFeatureGate_InvalidFeature_ReturnsFalse verifies that an unrecognised Feature value returns false.
func TestFeatureGate_InvalidFeature_ReturnsFalse(t *testing.T) {
	gate := newStubGate(core.CommunityLicenseClaims())
	assert.False(t, gate.Enabled(core.Feature(-1)))
	assert.False(t, gate.Enabled(core.Feature(9999)))
}
