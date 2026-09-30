package tuning

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSignFeedPayloadProducesVerifiableEnvelope(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(validFeedPayloadForTest())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := SignFeedPayload(payload, privateKey)
	if err != nil {
		t.Fatalf("SignFeedPayload: %v", err)
	}
	verified, err := verifySignedFeed(envelope, base64.StdEncoding.EncodeToString(publicKey))
	if err != nil {
		t.Fatalf("signed envelope did not verify: %v", err)
	}
	if verified.payload.Revision != validFeedPayloadForTest().Revision {
		t.Fatalf("revision=%d", verified.payload.Revision)
	}
}

func TestSignFeedPayloadRefusesInvalidPayload(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SignFeedPayload([]byte(`{"schemaVersion":1}`), privateKey); err == nil {
		t.Fatal("expected invalid payload to be rejected before signing")
	}
}
