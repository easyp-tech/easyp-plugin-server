package core

import (
	"testing"
)

func TestCommunityLicenseClaims(t *testing.T) {
	claims := CommunityLicenseClaims()

	if claims.Tier != LicenseTierCommunity {
		t.Errorf("expected tier %q, got %q", LicenseTierCommunity, claims.Tier)
	}

	if claims.MaxWorkers != communityMaxWorkers {
		t.Errorf("expected MaxWorkers=%d, got %d", communityMaxWorkers, claims.MaxWorkers)
	}

	if claims.MaxPlugins != communityMaxPlugins {
		t.Errorf("expected MaxPlugins=%d, got %d", communityMaxPlugins, claims.MaxPlugins)
	}

	// Verify no enterprise features are included.
	enterpriseFeatures := map[Feature]bool{
		FeatureMultiTenancy:    true,
		FeatureResponseCaching: true,
		FeatureAudit:           true,
	}
	for _, f := range claims.Features {
		if enterpriseFeatures[f] {
			t.Errorf("community defaults should not include enterprise feature %s", f)
		}
	}

	// Verify every non-enterprise feature is included.
	communityExpected := []Feature{
		FeatureCodeGeneration,
		FeaturePluginListing,
		FeatureMCPServerTools,
		FeatureRateLimiting,
		FeaturePluginCRUD,
	}
	featureSet := make(map[Feature]bool, len(claims.Features))
	for _, f := range claims.Features {
		featureSet[f] = true
	}
	for _, f := range communityExpected {
		if !featureSet[f] {
			t.Errorf("community defaults missing feature %s", f)
		}
	}
}

func TestCommunityLicenseClaims_FeatureCount(t *testing.T) {
	claims := CommunityLicenseClaims()

	// 5 community features: CodeGeneration, PluginListing, MCPServerTools, RateLimiting, PluginCRUD
	const expected = 5
	if len(claims.Features) != expected {
		t.Errorf("expected %d community features, got %d", expected, len(claims.Features))
	}
}

func TestLicenseTierConstants(t *testing.T) {
	if LicenseTierCommunity != "community" {
		t.Errorf("expected LicenseTierCommunity=%q, got %q", "community", LicenseTierCommunity)
	}
	if LicenseTierEnterprise != "enterprise" {
		t.Errorf("expected LicenseTierEnterprise=%q, got %q", "enterprise", LicenseTierEnterprise)
	}
}
