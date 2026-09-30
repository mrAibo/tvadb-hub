package tuning

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
