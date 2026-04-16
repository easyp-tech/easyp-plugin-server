package license

import "time"

// Tier определяет уровень лицензии.
type Tier string

const (
	TierCommunity  Tier = "community"
	TierEnterprise Tier = "enterprise"

	communityMaxWorkers = 4
	communityMaxPlugins = 10
)

// Claims содержит данные из PASETO-токена лицензии.
type Claims struct {
	Tier       Tier      `json:"tier"`
	Features   []feature `json:"features"`
	MaxWorkers int       `json:"max_workers"`
	MaxPlugins int       `json:"max_plugins"` // -1 = unlimited
	ExpiresAt  time.Time `json:"exp"`
	IssuedAt   time.Time `json:"iat"`
	Issuer     string    `json:"iss"`
	Subject    string    `json:"sub"`
	RefreshURL string    `json:"refresh_url,omitempty"`
}

// CommunityDefaults возвращает claims для Community-режима.
func CommunityDefaults() Claims {
	return Claims{
		Tier:       TierCommunity,
		Features:   communityFeatures(),
		MaxWorkers: communityMaxWorkers,
		MaxPlugins: communityMaxPlugins,
	}
}

// communityFeatures возвращает список всех не-Enterprise функций.
func communityFeatures() []feature {
	var features []feature
	for f := range featureCount {
		if !f.IsEnterprise() {
			features = append(features, f)
		}
	}

	return features
}
