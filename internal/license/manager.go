package license

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/prometheus/client_golang/prometheus"
)

// LicenseConfig содержит параметры конфигурации лицензии.
type LicenseConfig struct {
	Key  string `yaml:"key" env:"LICENSE_KEY"`
	File string `yaml:"file" env:"LICENSE_FILE"`
}

// LicenseManager отвечает за парсинг, валидацию и кэширование лицензии.
type LicenseManager struct {
	mu         sync.RWMutex
	claims     Claims
	valid      bool
	publicKey  paseto.V4AsymmetricPublicKey
	hasKey     bool
	logger     *slog.Logger
	metrics    *LicenseMetrics
	stopTicker chan struct{}
}

// NewLicenseManager создаёт LicenseManager.
// publicKeyHex — hex-encoded Ed25519 public key, встроенный через ldflags.
// Если publicKeyHex пуст, работает в Community-режиме.
func NewLicenseManager(
	publicKeyHex string,
	cfg LicenseConfig,
	logger *slog.Logger,
	reg *prometheus.Registry,
	namespace string,
) (*LicenseManager, error) {
	metrics := NewLicenseMetrics(reg, namespace)

	lm := &LicenseManager{
		claims:     CommunityDefaults(),
		logger:     logger,
		metrics:    metrics,
		stopTicker: make(chan struct{}),
	}

	// Step 1: If publicKeyHex is empty → Community mode.
	if publicKeyHex == "" {
		logger.Info("no public key provided, operating in community mode")
		lm.updateMetrics()
		return lm, nil
	}

	// Step 2: Decode publicKeyHex → paseto.V4AsymmetricPublicKey.
	pubKey, err := paseto.NewV4AsymmetricPublicKeyFromHex(publicKeyHex)
	if err != nil {
		logger.Error("failed to decode public key, operating in community mode", "error", err)
		lm.updateMetrics()
		return lm, fmt.Errorf("decode public key: %w", err)
	}
	lm.publicKey = pubKey
	lm.hasKey = true

	// Step 3: Load token.
	token, err := lm.loadToken(cfg)
	if err != nil {
		lm.updateMetrics()
		return lm, err
	}
	if token == "" {
		// No token configured → Community mode.
		logger.Info("no license token configured, operating in community mode")
		lm.updateMetrics()
		return lm, nil
	}

	// Step 4: If both key and file are specified → log warning.
	if cfg.Key != "" && cfg.File != "" {
		logger.Warn("both license.key and license.file specified, using license.key")
	}

	// Steps 5-9: Parse, validate, cache.
	if err := lm.applyToken(token); err != nil {
		lm.updateMetrics()
		return lm, err
	}

	lm.updateMetrics()
	return lm, nil
}

// Claims возвращает текущие кэшированные claims (потокобезопасно).
func (lm *LicenseManager) Claims() Claims {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.claims
}

// Valid возвращает true, если лицензия валидна.
func (lm *LicenseManager) Valid() bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.valid
}

// Reload загружает и валидирует новый токен, обновляя кэш.
func (lm *LicenseManager) Reload(token string) error {
	if !lm.hasKey {
		return fmt.Errorf("cannot reload: no public key configured")
	}

	if err := lm.applyToken(token); err != nil {
		return err
	}

	lm.updateMetrics()
	return nil
}

// ParseToken парсит PASETO v4.public токен и возвращает Claims.
func (lm *LicenseManager) ParseToken(token string) (Claims, error) {
	if !lm.hasKey {
		return Claims{}, fmt.Errorf("%w: no public key configured", ErrInvalidToken)
	}

	parser := paseto.NewParserWithoutExpiryCheck()
	parsed, err := parser.ParseV4Public(lm.publicKey, token, nil)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %s", ErrSignatureInvalid, err.Error())
	}

	claims, err := extractClaims(parsed)
	if err != nil {
		return Claims{}, err
	}

	return claims, nil
}

// FormatToken форматирует Claims в строковое представление PASETO-токена.
// Требует приватный ключ (используется только в тестах и CLI для генерации лицензий).
func FormatToken(claims Claims, privateKey paseto.V4AsymmetricSecretKey) (string, error) {
	token := paseto.NewToken()

	if err := token.Set("tier", claims.Tier); err != nil {
		return "", fmt.Errorf("set tier: %w", err)
	}
	if err := token.Set("features", claims.Features); err != nil {
		return "", fmt.Errorf("set features: %w", err)
	}
	if err := token.Set("max_workers", claims.MaxWorkers); err != nil {
		return "", fmt.Errorf("set max_workers: %w", err)
	}
	if err := token.Set("max_plugins", claims.MaxPlugins); err != nil {
		return "", fmt.Errorf("set max_plugins: %w", err)
	}

	token.SetExpiration(claims.ExpiresAt)
	token.SetIssuedAt(claims.IssuedAt)
	token.SetIssuer(claims.Issuer)
	token.SetSubject(claims.Subject)

	if claims.RefreshURL != "" {
		token.SetString("refresh_url", claims.RefreshURL)
	}

	signed := token.V4Sign(privateKey, nil)
	return signed, nil
}

// StartExpirationWatcher запускает горутину с тикером (60s),
// проверяющую истечение лицензии.
func (lm *LicenseManager) StartExpirationWatcher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.checkExpiration()
			case <-lm.stopTicker:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop останавливает тикер проверки истечения.
func (lm *LicenseManager) Stop() {
	select {
	case lm.stopTicker <- struct{}{}:
	default:
	}
}

// Metrics возвращает метрики лицензирования (для использования в FeatureGate).
func (lm *LicenseManager) Metrics() *LicenseMetrics {
	return lm.metrics
}

// loadToken загружает токен из конфигурации.
func (lm *LicenseManager) loadToken(cfg LicenseConfig) (string, error) {
	if cfg.Key != "" {
		return cfg.Key, nil
	}

	if cfg.File != "" {
		data, err := os.ReadFile(cfg.File)
		if err != nil {
			if os.IsNotExist(err) {
				lm.logger.Error("license file not found, operating in community mode", "file", cfg.File)
				return "", fmt.Errorf("%w: %s", ErrFileNotFound, cfg.File)
			}
			return "", fmt.Errorf("read license file: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}

// applyToken парсит и применяет токен, обновляя кэш.
func (lm *LicenseManager) applyToken(token string) error {
	claims, err := lm.ParseToken(token)
	if err != nil {
		lm.logger.Error("failed to parse license token, operating in community mode", "error", err)
		lm.mu.Lock()
		lm.claims = CommunityDefaults()
		lm.valid = false
		lm.mu.Unlock()
		return err
	}

	// Check expiration.
	if time.Now().After(claims.ExpiresAt) {
		lm.logger.Warn("license token expired, operating in community mode",
			"expired_at", claims.ExpiresAt.Format(time.RFC3339))
		lm.mu.Lock()
		lm.claims = CommunityDefaults()
		lm.valid = false
		lm.mu.Unlock()
		return ErrTokenExpired
	}

	// Cache claims.
	lm.mu.Lock()
	lm.claims = claims
	lm.valid = true
	lm.mu.Unlock()

	lm.logger.Info("license loaded successfully",
		"tier", string(claims.Tier),
		"expires_at", claims.ExpiresAt.Format(time.RFC3339),
		"features_count", len(claims.Features))

	return nil
}

// checkExpiration проверяет истечение лицензии.
func (lm *LicenseManager) checkExpiration() {
	lm.mu.RLock()
	expiresAt := lm.claims.ExpiresAt
	valid := lm.valid
	lm.mu.RUnlock()

	if !valid {
		return
	}

	if time.Now().After(expiresAt) {
		lm.mu.Lock()
		// Double-check after acquiring write lock.
		if lm.valid {
			lm.claims = CommunityDefaults()
			lm.valid = false
			lm.logger.Warn("license expired during runtime, transitioning to community mode",
				"expired_at", expiresAt.Format(time.RFC3339))
		}
		lm.mu.Unlock()
		lm.updateMetrics()
	}
}

// updateMetrics обновляет Prometheus-метрики.
func (lm *LicenseManager) updateMetrics() {
	lm.mu.RLock()
	valid := lm.valid
	expiresAt := lm.claims.ExpiresAt
	lm.mu.RUnlock()

	if valid {
		lm.metrics.valid.Set(1)
	} else {
		lm.metrics.valid.Set(0)
	}

	if !expiresAt.IsZero() {
		lm.metrics.expiryTS.Set(float64(expiresAt.Unix()))
	}
}

// extractClaims извлекает Claims из распарсенного PASETO-токена.
func extractClaims(token *paseto.Token) (Claims, error) {
	claimsJSON := token.ClaimsJSON()

	// We use an intermediate struct for JSON unmarshalling since the PASETO
	// library stores time as RFC3339 strings, not as time.Time directly.
	var raw struct {
		Tier       Tier      `json:"tier"`
		Features   []feature `json:"features"`
		MaxWorkers int       `json:"max_workers"`
		MaxPlugins int       `json:"max_plugins"`
		RefreshURL string    `json:"refresh_url"`
	}
	if err := json.Unmarshal(claimsJSON, &raw); err != nil {
		return Claims{}, fmt.Errorf("%w: %s", ErrInvalidClaims, err.Error())
	}

	// Extract time fields using the PASETO token's built-in methods (RFC3339).
	exp, err := token.GetExpiration()
	if err != nil {
		return Claims{}, fmt.Errorf("%w: missing exp: %s", ErrInvalidClaims, err.Error())
	}

	iat, err := token.GetIssuedAt()
	if err != nil {
		// IssuedAt is optional for parsing; use zero time if missing.
		iat = time.Time{}
	}

	iss, _ := token.GetIssuer()
	sub, _ := token.GetSubject()

	claims := Claims{
		Tier:       raw.Tier,
		Features:   raw.Features,
		MaxWorkers: raw.MaxWorkers,
		MaxPlugins: raw.MaxPlugins,
		ExpiresAt:  exp,
		IssuedAt:   iat,
		Issuer:     iss,
		Subject:    sub,
		RefreshURL: raw.RefreshURL,
	}

	return claims, nil
}
