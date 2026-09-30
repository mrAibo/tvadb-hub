package tuning

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func testFeedProfile() Profile {
	return Profile{
		ID:            "signed-test",
		Name:          "Signed test profile",
		DeviceFamily:  "Test Android",
		Description:   "A conservative signed metadata test profile.",
		SourceName:    "DroidSphere tests",
		SourceURL:     "https://example.com/source",
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"example"}},
		Keep:          []string{"com.example.core"},
		Rules: []PackageRule{
			{
				PackageName:     "com.example.optional",
				Label:           "Optional app",
				Category:        "Optional",
				Risk:            RiskSafe,
				Reason:          "Test-only optional package.",
				DefaultSelected: true,
			},
		},
	}
}

func signedEnvelopeForTest(t *testing.T, payload FeedPayload) ([]byte, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope := SignedFeedEnvelope{
		Format:    feedEnvelopeFormat,
		Version:   feedEnvelopeVersion,
		KeyID:     feedKeyID(publicKey),
		Payload:   base64.StdEncoding.EncodeToString(payloadBytes),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payloadBytes)),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data, base64.StdEncoding.EncodeToString(publicKey)
}

func validFeedPayloadForTest() FeedPayload {
	return FeedPayload{
		SchemaVersion: feedSchemaVersion,
		Revision:      1,
		Version:       "2026.09.30",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		SourceName:    "DroidSphere test feed",
		SourceURL:     "https://example.com/feed-source",
		SourceLicense: "MIT",
		Profiles:      []Profile{testFeedProfile()},
	}
}

func TestVerifySignedFeedAcceptsValidEnvelope(t *testing.T) {
	data, publicKey := signedEnvelopeForTest(t, validFeedPayloadForTest())
	verified, err := verifySignedFeed(data, publicKey)
	if err != nil {
		t.Fatalf("verifySignedFeed returned error: %v", err)
	}
	if verified.payload.Revision != 1 || len(verified.payload.Profiles) != 1 {
		t.Fatalf("unexpected verified payload: %+v", verified.payload)
	}
}

func TestVerifySignedFeedRejectsTamperedPayload(t *testing.T) {
	payload := validFeedPayloadForTest()
	data, publicKey := signedEnvelopeForTest(t, payload)

	var envelope SignedFeedEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-2] ^= 1
	envelope.Payload = base64.StdEncoding.EncodeToString(raw)
	tampered, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := verifySignedFeed(tampered, publicKey); err == nil {
		t.Fatal("expected tampered payload to fail signature verification")
	}
}

func TestVerifySignedFeedRejectsWrongKey(t *testing.T) {
	data, _ := signedEnvelopeForTest(t, validFeedPayloadForTest())
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifySignedFeed(data, base64.StdEncoding.EncodeToString(otherPublic)); err == nil {
		t.Fatal("expected wrong public key to be rejected")
	}
}

func TestValidateFeedPayloadRejectsUnsafeDefaultSelection(t *testing.T) {
	payload := validFeedPayloadForTest()
	payload.Profiles[0].Rules[0].Risk = RiskCaution
	if err := validateFeedPayload(payload); err == nil {
		t.Fatal("expected caution default selection to be rejected")
	}
}

func TestValidateFeedPayloadRequiresSourceLicense(t *testing.T) {
	payload := validFeedPayloadForTest()
	payload.Profiles[0].SourceLicense = ""
	if err := validateFeedPayload(payload); err == nil {
		t.Fatal("expected missing source license to be rejected")
	}
}

func TestMergeProfilesAllowsSignedOverlayWithoutMutatingBuiltins(t *testing.T) {
	base := []Profile{{ID: "same", Name: "Builtin"}, {ID: "other", Name: "Other"}}
	overlay := []Profile{{ID: "same", Name: "Signed"}, {ID: "new", Name: "New"}}
	merged := mergeProfiles(base, overlay)
	if len(merged) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(merged))
	}
	if merged[0].Name != "Signed" || base[0].Name != "Builtin" {
		t.Fatalf("overlay did not replace safely: merged=%+v base=%+v", merged[0], base[0])
	}
}

func TestNormalizeFeedConfigRequiresPairAndHTTPS(t *testing.T) {
	if _, err := NormalizeFeedConfig(FeedConfig{URL: "https://example.com/feed.json"}); err == nil {
		t.Fatal("expected URL without key to fail")
	}
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(publicKey)
	if _, err := NormalizeFeedConfig(FeedConfig{URL: "http://example.com/feed.json", PublicKey: key}); err == nil {
		t.Fatal("expected non-HTTPS URL to fail")
	}
	if _, err := NormalizeFeedConfig(FeedConfig{URL: "https://example.com/feed.json", PublicKey: key}); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}

func TestValidateFeedPayloadRejectsMissingFeedLicense(t *testing.T) {
	payload := validFeedPayloadForTest()
	payload.SourceLicense = ""
	if err := validateFeedPayload(payload); err == nil {
		t.Fatal("expected missing feed-level source license to be rejected")
	}
}

func TestValidateFeedPayloadRejectsHardProtectedActionableRule(t *testing.T) {
	payload := validFeedPayloadForTest()
	payload.Profiles[0].Rules[0] = PackageRule{
		PackageName:     "com.android.systemui",
		Label:           "System UI",
		Category:        "Core",
		Risk:            RiskSafe,
		Reason:          "A signed feed must not make this actionable.",
		DefaultSelected: true,
	}
	if err := validateFeedPayload(payload); err == nil {
		t.Fatal("expected hard-protected actionable rule to be rejected")
	}
}

func TestFeedCacheNamespaceBindsURLAndPublicKey(t *testing.T) {
	manager := newFeedManager(t.TempDir())
	publicKeyA, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyB, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	configA := FeedConfig{URL: "https://example.com/a.json", PublicKey: base64.StdEncoding.EncodeToString(publicKeyA)}
	configB := FeedConfig{URL: "https://example.com/b.json", PublicKey: base64.StdEncoding.EncodeToString(publicKeyB)}
	if manager.currentPathFor(configA) == manager.currentPathFor(configB) {
		t.Fatal("different trust configurations must not share a metadata cache")
	}
}

func TestPromoteCachePreservesVerifiedPreviousWhenCurrentIsInvalid(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := FeedConfig{
		URL:       "https://example.com/feed.json",
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
	}
	manager := newFeedManager(t.TempDir())
	if err := os.MkdirAll(manager.cacheDirFor(config), 0o700); err != nil {
		t.Fatal(err)
	}

	previousPayload := validFeedPayloadForTest()
	previousPayload.Revision = 1
	previousPayload.Version = "1"
	previousBytes, err := json.Marshal(previousPayload)
	if err != nil {
		t.Fatal(err)
	}
	previousEnvelope, err := SignFeedPayload(previousBytes, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	newPayload := validFeedPayloadForTest()
	newPayload.Revision = 2
	newPayload.Version = "2"
	newBytes, err := json.Marshal(newPayload)
	if err != nil {
		t.Fatal(err)
	}
	newEnvelope, err := SignFeedPayload(newBytes, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(manager.previousPathFor(config), previousEnvelope, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.currentPathFor(config), []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := manager.promoteCache(newEnvelope, config); err != nil {
		t.Fatalf("promoteCache: %v", err)
	}

	preservedPrevious, err := os.ReadFile(manager.previousPathFor(config))
	if err != nil {
		t.Fatal(err)
	}
	verifiedPrevious, err := verifySignedFeed(preservedPrevious, config.PublicKey)
	if err != nil {
		t.Fatalf("previous cache should remain valid: %v", err)
	}
	if verifiedPrevious.payload.Revision != 1 {
		t.Fatalf("previous revision=%d, want 1", verifiedPrevious.payload.Revision)
	}

	current, err := os.ReadFile(manager.currentPathFor(config))
	if err != nil {
		t.Fatal(err)
	}
	verifiedCurrent, err := verifySignedFeed(current, config.PublicKey)
	if err != nil {
		t.Fatalf("current cache should be replaced with verified metadata: %v", err)
	}
	if verifiedCurrent.payload.Revision != 2 {
		t.Fatalf("current revision=%d, want 2", verifiedCurrent.payload.Revision)
	}
}

