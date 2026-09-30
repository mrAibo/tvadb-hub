package tuning

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// SignFeedPayload validates a raw feed payload before signing it and returns
// a complete DroidSphere feed envelope. It is intended for the maintainer
// signing tool; private keys are never stored by the application.
func SignFeedPayload(payloadBytes []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("Ed25519 private key must be %d bytes", ed25519.PrivateKeySize)
	}

	var payload FeedPayload
	if err := decodeStrictJSON(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid feed payload: %w", err)
	}
	if err := validateFeedPayload(payload); err != nil {
		return nil, err
	}

	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("failed to derive Ed25519 public key")
	}

	envelope := SignedFeedEnvelope{
		Format:    feedEnvelopeFormat,
		Version:   feedEnvelopeVersion,
		KeyID:     feedKeyID(publicKey),
		Payload:   base64.StdEncoding.EncodeToString(payloadBytes),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payloadBytes)),
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode signed feed envelope: %w", err)
	}
	return append(data, '\n'), nil
}
