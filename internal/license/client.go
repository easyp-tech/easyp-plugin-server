package license

import (
	"context"

	"github.com/easyp-tech/service/internal/core"
)

// MockLicenseClient is a temporary implementation of core.LicenseClient.
// TODO: replace with a real gRPC client when the license server is available.
type MockLicenseClient struct{}

// NewMockLicenseClient creates a MockLicenseClient.
func NewMockLicenseClient() *MockLicenseClient {
	return &MockLicenseClient{}
}

// ValidateLicense always returns an Enterprise license without any network call.
func (m *MockLicenseClient) ValidateLicense(_ context.Context) (core.LicenseClaims, error) {
	return core.LicenseClaims{
		Tier: core.LicenseTierEnterprise,
		Features: []core.Feature{
			core.FeatureCodeGeneration,
			core.FeaturePluginListing,
			core.FeatureMCPServerTools,
			core.FeatureRateLimiting,
			core.FeaturePluginCRUD,
			core.FeatureMultiTenancy,
			core.FeatureResponseCaching,
			core.FeatureAudit,
		},
		MaxWorkers: -1,
		MaxPlugins: -1,
	}, nil
}
