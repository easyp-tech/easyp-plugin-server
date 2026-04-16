package license

import (
	"testing"
)

func TestCommunityDefaults(t *testing.T) {
	claims := CommunityDefaults()

	if claims.Tier != TierCommunity {
		t.Errorf("expected tier %q, got %q", TierCommunity, claims.Tier)
	}

	if claims.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers=4, got %d", claims.MaxWorkers)
	}

	if claims.MaxPlugins != 10 {
		t.Errorf("expected MaxPlugins=10, got %d", claims.MaxPlugins)
	}

	// Verify all community features are present and no enterprise features.
	for _, f := range claims.Features {
		if f.IsEnterprise() {
			t.Errorf("community defaults should not include enterprise feature %s", f)
		}
	}

	// Verify every non-enterprise feature is included.
	featureSet := make(map[feature]bool, len(claims.Features))
	for _, f := range claims.Features {
		featureSet[f] = true
	}

	for f := range featureCount {
		if !f.IsEnterprise() && !featureSet[f] {
			t.Errorf("community defaults missing feature %s", f)
		}
		if f.IsEnterprise() && featureSet[f] {
			t.Errorf("community defaults should not contain enterprise feature %s", f)
		}
	}
}

func TestCommunityDefaults_FeatureCount(t *testing.T) {
	claims := CommunityDefaults()

	// Count expected community features.
	var expected int
	for f := range featureCount {
		if !f.IsEnterprise() {
			expected++
		}
	}

	if len(claims.Features) != expected {
		t.Errorf("expected %d community features, got %d", expected, len(claims.Features))
	}
}

func TestTierConstants(t *testing.T) {
	if TierCommunity != "community" {
		t.Errorf("expected TierCommunity=%q, got %q", "community", TierCommunity)
	}
	if TierEnterprise != "enterprise" {
		t.Errorf("expected TierEnterprise=%q, got %q", "enterprise", TierEnterprise)
	}
}
