package tuning

import (
	"ADBKit/internal/core"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	feedEnvelopeFormat  = "droidsphere-safe-tuning-feed"
	feedEnvelopeVersion = 1
	feedSchemaVersion   = 1
	maxFeedBytes        = 4 << 20
	maxFeedProfiles     = 256
	maxRulesPerProfile  = 5000
)

var (
	profileIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	packageNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)+$`)
)

type FeedConfig struct {
	URL       string `json:"url"`
	PublicKey string `json:"publicKey"`
}

type FeedStatus struct {
	Configured   bool   `json:"configured"`
	Active       bool   `json:"active"`
	Source       string `json:"source"`
	URL          string `json:"url"`
	Version      string `json:"version"`
	Revision     uint64 `json:"revision"`
	GeneratedAt  string `json:"generatedAt"`
	KeyID        string `json:"keyId"`
	ProfileCount int    `json:"profileCount"`
	CanRollback  bool   `json:"canRollback"`
	Message      string `json:"message"`
}

type SignedFeedEnvelope struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	KeyID     string `json:"keyId"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type FeedPayload struct {
	SchemaVersion int       `json:"schemaVersion"`
	Revision      uint64    `json:"revision"`
	Version       string    `json:"version"`
	GeneratedAt   string    `json:"generatedAt"`
	Profiles      []Profile `json:"profiles"`
}

type verifiedFeed struct {
	payload FeedPayload
	digest  string
	keyID   string
}

type feedManager struct {
	mu sync.RWMutex

	dataDir  string
	resolver func() FeedConfig
	client   *http.Client

	configFingerprint string
	config            FeedConfig
	profiles          []Profile
	status            FeedStatus
	activeDigest      string
}

func newFeedManager(dataDir string) *feedManager {
	return &feedManager{
		dataDir: dataDir,
		client:  newFeedHTTPClient(),
		status: FeedStatus{
			Source:  "builtin",
			Message: "Using the built-in Safe Tuning catalog.",
		},
	}
}

func newFeedHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validateFeedURL(req.URL.String())
		},
	}
}

func (m *feedManager) setResolver(resolver func() FeedConfig) {
	m.mu.Lock()
	m.resolver = resolver
	m.configFingerprint = ""
	m.mu.Unlock()
	m.ensureConfigured()
}

func NormalizeFeedConfig(config FeedConfig) (FeedConfig, error) {
	config.URL = strings.TrimSpace(config.URL)
	config.PublicKey = strings.TrimSpace(config.PublicKey)
	if config.URL == "" && config.PublicKey == "" {
		return config, nil
	}
	if config.URL == "" || config.PublicKey == "" {
		return FeedConfig{}, core.NewOperationError(
			"safe_tuning_feed_config",
			"Safe Tuning feed URL and public key must be configured together",
			"both values are required, or leave both blank to use only the built-in catalog",
			false,
		)
	}
	if err := validateFeedURL(config.URL); err != nil {
		return FeedConfig{}, core.NewOperationError(
			"safe_tuning_feed_config",
			"Safe Tuning feed URL is invalid",
			err.Error(),
			false,
		)
	}
	if _, err := decodePublicKey(config.PublicKey); err != nil {
		return FeedConfig{}, core.NewOperationError(
			"safe_tuning_feed_config",
			"Safe Tuning feed public key is invalid",
			err.Error(),
			false,
		)
	}
	return config, nil
}

func validateFeedURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("feed URL must use HTTPS")
	}
	if parsed.Host == "" {
		return errors.New("feed URL must include a host")
	}
	if parsed.User != nil {
		return errors.New("credentials are not allowed in the feed URL")
	}
	return nil
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("public key must be standard base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must decode to %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func feedKeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:8])
}

func configFingerprint(config FeedConfig) string {
	sum := sha256.Sum256([]byte(config.URL + "\x00" + config.PublicKey))
	return hex.EncodeToString(sum[:])
}

func (m *feedManager) ensureConfigured() {
	m.mu.RLock()
	resolver := m.resolver
	currentFingerprint := m.configFingerprint
	m.mu.RUnlock()
	if resolver == nil {
		return
	}

	config, err := NormalizeFeedConfig(resolver())
	fingerprint := ""
	if err == nil {
		fingerprint = configFingerprint(config)
	} else {
		fingerprint = "invalid:" + err.Error()
	}
	if fingerprint == currentFingerprint {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if fingerprint == m.configFingerprint {
		return
	}
	m.configFingerprint = fingerprint
	m.profiles = nil
	m.activeDigest = ""

	if err != nil {
		m.config = FeedConfig{}
		m.status = FeedStatus{
			Source:  "builtin",
			Message: err.Error(),
		}
		return
	}
	m.config = config
	if config.URL == "" {
		m.status = FeedStatus{
			Source:  "builtin",
			Message: "Using the built-in Safe Tuning catalog.",
		}
		return
	}
	m.loadCachedLocked()
}

func (m *feedManager) statusSnapshot() FeedStatus {
	m.ensureConfigured()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *feedManager) activeProfiles() []Profile {
	m.ensureConfigured()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return mergeProfiles(builtinProfiles, m.profiles)
}

func mergeProfiles(base, overlay []Profile) []Profile {
	out := make([]Profile, len(base))
	copy(out, base)
	index := make(map[string]int, len(out))
	for i := range out {
		index[out[i].ID] = i
	}
	for _, profile := range overlay {
		if i, ok := index[profile.ID]; ok {
			out[i] = profile
			continue
		}
		index[profile.ID] = len(out)
		out = append(out, profile)
	}
	return out
}

func (m *feedManager) loadCachedLocked() {
	currentData, currentErr := os.ReadFile(m.currentPath())
	if currentErr == nil {
		if verified, err := verifySignedFeed(currentData, m.config.PublicKey); err == nil {
			m.activateLocked(verified, "cache", "Loaded the verified Safe Tuning metadata cache.")
			m.status.CanRollback = m.validPreviousLocked()
			return
		}
	}

	previousData, previousErr := os.ReadFile(m.previousPath())
	if previousErr == nil {
		if verified, err := verifySignedFeed(previousData, m.config.PublicKey); err == nil {
			m.activateLocked(verified, "rollback-cache", "Current metadata cache was unavailable or invalid; using the verified previous cache.")
			m.status.CanRollback = false
			return
		}
	}

	m.status = FeedStatus{
		Configured: true,
		Source:     "builtin",
		URL:        m.config.URL,
		KeyID:      configuredKeyID(m.config.PublicKey),
		Message:    "Signed metadata feed is configured but no verified cache is active. Refresh to download it; built-in profiles remain in use.",
	}
}

func (m *feedManager) activateLocked(verified verifiedFeed, source, message string) {
	profiles := make([]Profile, len(verified.payload.Profiles))
	copy(profiles, verified.payload.Profiles)
	m.profiles = profiles
	m.activeDigest = verified.digest
	m.status = FeedStatus{
		Configured:   true,
		Active:       true,
		Source:       source,
		URL:          m.config.URL,
		Version:      verified.payload.Version,
		Revision:     verified.payload.Revision,
		GeneratedAt:  verified.payload.GeneratedAt,
		KeyID:        verified.keyID,
		ProfileCount: len(profiles),
		Message:      message,
	}
}

func configuredKeyID(encoded string) string {
	key, err := decodePublicKey(encoded)
	if err != nil {
		return ""
	}
	return feedKeyID(key)
}

func (m *feedManager) refresh(ctx context.Context) (FeedStatus, error) {
	m.ensureConfigured()

	m.mu.RLock()
	config := m.config
	activeRevision := m.status.Revision
	activeDigest := m.activeDigest
	client := m.client
	m.mu.RUnlock()

	if config.URL == "" {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_refresh",
			"Safe Tuning metadata feed is not configured",
			"configure an HTTPS feed URL and Ed25519 public key first",
			false,
		)
	}

	data, err := fetchFeed(ctx, client, config.URL)
	if err != nil {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_refresh",
			"Failed to download Safe Tuning metadata",
			err.Error(),
			true,
		)
	}
	verified, err := verifySignedFeed(data, config.PublicKey)
	if err != nil {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_refresh",
			"Safe Tuning metadata signature or payload is invalid",
			err.Error(),
			false,
		)
	}
	if activeRevision > 0 {
		if verified.payload.Revision < activeRevision {
			return m.statusSnapshot(), core.NewOperationError(
				"safe_tuning_feed_refresh",
				"Safe Tuning metadata revision is older than the active revision",
				fmt.Sprintf("received revision %d, active revision %d; use explicit rollback for older cached metadata", verified.payload.Revision, activeRevision),
				false,
			)
		}
		if verified.payload.Revision == activeRevision && activeDigest != "" && verified.digest != activeDigest {
			return m.statusSnapshot(), core.NewOperationError(
				"safe_tuning_feed_refresh",
				"Safe Tuning metadata revision collision detected",
				fmt.Sprintf("revision %d has different signed content", verified.payload.Revision),
				false,
			)
		}
		if verified.payload.Revision == activeRevision && verified.digest == activeDigest {
			status := m.statusSnapshot()
			status.Message = "The active signed Safe Tuning metadata is already current."
			return status, nil
		}
	}

	if err := m.promoteCache(data); err != nil {
		return m.statusSnapshot(), err
	}

	m.mu.Lock()
	m.activateLocked(verified, "remote", "Downloaded and activated verified Safe Tuning metadata.")
	m.status.CanRollback = m.validPreviousLocked()
	status := m.status
	m.mu.Unlock()
	return status, nil
}

func fetchFeed(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := validateFeedURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed server returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFeedBytes {
		return nil, fmt.Errorf("feed exceeds the %d byte limit", maxFeedBytes)
	}
	return data, nil
}

func verifySignedFeed(data []byte, publicKeyText string) (verifiedFeed, error) {
	publicKey, err := decodePublicKey(publicKeyText)
	if err != nil {
		return verifiedFeed{}, err
	}

	var envelope SignedFeedEnvelope
	if err := decodeStrictJSON(data, &envelope); err != nil {
		return verifiedFeed{}, fmt.Errorf("invalid feed envelope: %w", err)
	}
	if envelope.Format != feedEnvelopeFormat || envelope.Version != feedEnvelopeVersion {
		return verifiedFeed{}, fmt.Errorf("unsupported feed envelope %q version %d", envelope.Format, envelope.Version)
	}
	expectedKeyID := feedKeyID(publicKey)
	if envelope.KeyID != expectedKeyID {
		return verifiedFeed{}, fmt.Errorf("feed key id %q does not match configured key %q", envelope.KeyID, expectedKeyID)
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return verifiedFeed{}, fmt.Errorf("payload is not valid base64: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		return verifiedFeed{}, fmt.Errorf("signature is not valid base64: %w", err)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, payloadBytes, signature) {
		return verifiedFeed{}, errors.New("Ed25519 signature verification failed")
	}

	var payload FeedPayload
	if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
		return verifiedFeed{}, fmt.Errorf("invalid signed payload: %w", err)
	}
	if err := validateFeedPayload(payload); err != nil {
		return verifiedFeed{}, err
	}
	digest := sha256.Sum256(payloadBytes)
	return verifiedFeed{
		payload: payload,
		digest:  hex.EncodeToString(digest[:]),
		keyID:   expectedKeyID,
	}, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("unexpected data after JSON document")
	}
	return nil
}

func validateFeedPayload(payload FeedPayload) error {
	if payload.SchemaVersion != feedSchemaVersion {
		return fmt.Errorf("unsupported Safe Tuning feed schema version %d", payload.SchemaVersion)
	}
	if payload.Revision == 0 {
		return errors.New("feed revision must be greater than zero")
	}
	if strings.TrimSpace(payload.Version) == "" {
		return errors.New("feed version is required")
	}
	if _, err := time.Parse(time.RFC3339, payload.GeneratedAt); err != nil {
		return fmt.Errorf("feed generatedAt must be RFC3339: %w", err)
	}
	if len(payload.Profiles) == 0 || len(payload.Profiles) > maxFeedProfiles {
		return fmt.Errorf("feed must contain 1..%d profiles", maxFeedProfiles)
	}
	seen := make(map[string]struct{}, len(payload.Profiles))
	for i, profile := range payload.Profiles {
		if _, ok := seen[profile.ID]; ok {
			return fmt.Errorf("profile %q is duplicated", profile.ID)
		}
		seen[profile.ID] = struct{}{}
		if err := validateFeedProfile(profile); err != nil {
			return fmt.Errorf("profile %d (%s): %w", i, profile.ID, err)
		}
	}
	return nil
}

func validateFeedProfile(profile Profile) error {
	if !profileIDPattern.MatchString(profile.ID) {
		return errors.New("profile id is invalid")
	}
	if strings.TrimSpace(profile.Name) == "" || strings.TrimSpace(profile.DeviceFamily) == "" ||
		strings.TrimSpace(profile.Description) == "" {
		return errors.New("name, deviceFamily and description are required")
	}
	if strings.TrimSpace(profile.SourceName) == "" || strings.TrimSpace(profile.SourceLicense) == "" {
		return errors.New("sourceName and sourceLicense are required")
	}
	if err := validateSourceURL(profile.SourceURL); err != nil {
		return err
	}
	if !profile.Criteria.Generic &&
		len(profile.Criteria.Manufacturers) == 0 &&
		len(profile.Criteria.Brands) == 0 &&
		len(profile.Criteria.Models) == 0 &&
		len(profile.Criteria.Codenames) == 0 {
		return errors.New("profile criteria must be generic or include a device match field")
	}
	if len(profile.Rules) == 0 || len(profile.Rules) > maxRulesPerProfile {
		return fmt.Errorf("profile must contain 1..%d rules", maxRulesPerProfile)
	}

	keep := make(map[string]struct{}, len(profile.Keep))
	for _, pkg := range profile.Keep {
		pkg = strings.TrimSpace(pkg)
		if !validPackageName(pkg) {
			return fmt.Errorf("keep package %q is invalid", pkg)
		}
		if _, exists := keep[pkg]; exists {
			return fmt.Errorf("keep package %q is duplicated", pkg)
		}
		keep[pkg] = struct{}{}
	}

	rules := make(map[string]struct{}, len(profile.Rules))
	for _, rule := range profile.Rules {
		pkg := strings.TrimSpace(rule.PackageName)
		if !validPackageName(pkg) {
			return fmt.Errorf("rule package %q is invalid", pkg)
		}
		if _, exists := rules[pkg]; exists {
			return fmt.Errorf("rule package %q is duplicated", pkg)
		}
		rules[pkg] = struct{}{}
		if strings.TrimSpace(rule.Label) == "" || strings.TrimSpace(rule.Category) == "" || strings.TrimSpace(rule.Reason) == "" {
			return fmt.Errorf("rule %q requires label, category and reason", pkg)
		}
		switch rule.Risk {
		case RiskSafe:
		case RiskCaution:
			if rule.DefaultSelected {
				return fmt.Errorf("caution rule %q cannot be selected by default", pkg)
			}
		case RiskDangerous, RiskBlocked:
			if rule.DefaultSelected {
				return fmt.Errorf("high-risk rule %q cannot be selected by default", pkg)
			}
		default:
			return fmt.Errorf("rule %q has unsupported risk %q", pkg, rule.Risk)
		}
		if _, protected := keep[pkg]; protected && rule.Risk != RiskBlocked {
			return fmt.Errorf("package %q cannot be both kept and actionable", pkg)
		}
	}
	return nil
}

func validateSourceURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("sourceUrl must be a valid HTTPS URL")
	}
	return nil
}

func validPackageName(value string) bool {
	return len(value) <= 255 && packageNamePattern.MatchString(value)
}

func (m *feedManager) promoteCache(data []byte) error {
	dir := m.cacheDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return core.NewOperationError("safe_tuning_feed_cache", "Failed to create Safe Tuning metadata cache", err.Error(), true)
	}
	if current, err := os.ReadFile(m.currentPath()); err == nil {
		if err := core.WriteFileAtomicWithMode(m.previousPath(), current, 0o600); err != nil {
			return err
		}
	}
	if err := core.WriteFileAtomicWithMode(m.currentPath(), data, 0o600); err != nil {
		return err
	}
	return nil
}

func (m *feedManager) rollback() (FeedStatus, error) {
	m.ensureConfigured()

	m.mu.RLock()
	config := m.config
	m.mu.RUnlock()
	if config.URL == "" {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_rollback",
			"Safe Tuning metadata feed is not configured",
			"",
			false,
		)
	}

	previousData, err := os.ReadFile(m.previousPath())
	if err != nil {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_rollback",
			"No previous Safe Tuning metadata cache is available",
			err.Error(),
			false,
		)
	}
	previous, err := verifySignedFeed(previousData, config.PublicKey)
	if err != nil {
		return m.statusSnapshot(), core.NewOperationError(
			"safe_tuning_feed_rollback",
			"Previous Safe Tuning metadata cache is invalid",
			err.Error(),
			false,
		)
	}

	currentData, currentErr := os.ReadFile(m.currentPath())
	currentValid := false
	if currentErr == nil {
		_, currentVerifyErr := verifySignedFeed(currentData, config.PublicKey)
		currentValid = currentVerifyErr == nil
	}
	if err := core.WriteFileAtomicWithMode(m.currentPath(), previousData, 0o600); err != nil {
		return m.statusSnapshot(), err
	}
	if currentValid {
		if err := core.WriteFileAtomicWithMode(m.previousPath(), currentData, 0o600); err != nil {
			return m.statusSnapshot(), err
		}
	}

	m.mu.Lock()
	m.activateLocked(previous, "rollback", "Rolled back to the previous verified Safe Tuning metadata revision.")
	m.status.CanRollback = currentValid
	status := m.status
	m.mu.Unlock()
	return status, nil
}

func (m *feedManager) validPreviousLocked() bool {
	data, err := os.ReadFile(m.previousPath())
	if err != nil {
		return false
	}
	_, err = verifySignedFeed(data, m.config.PublicKey)
	return err == nil
}

func (m *feedManager) cacheDir() string {
	return filepath.Join(m.dataDir, "safe-tuning-feed")
}

func (m *feedManager) currentPath() string {
	return filepath.Join(m.cacheDir(), "current.json")
}

func (m *feedManager) previousPath() string {
	return filepath.Join(m.cacheDir(), "previous.json")
}
