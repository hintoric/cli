// Package signer wraps ECDSA P-256 sign/verify with PEM-encoded key helpers.
//
// The host embeds an ECDSA P-256 public key and verifies plugin manifests with
// it. Production releases sign via AWS KMS asymmetric keys (the private key
// never leaves the HSM); local development can use a plaintext PEM private key
// via Sign / ParsePrivateKeyPEM.
package signer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// Sign returns an ASN.1 DER-encoded ECDSA-SHA256 signature of msg using priv.
// This is the same wire format AWS KMS produces with SigningAlgorithm
// ECDSA_SHA_256, so signatures are interchangeable with KMS-produced ones.
func Sign(priv *ecdsa.PrivateKey, msg []byte) ([]byte, error) {
	digest := sha256.Sum256(msg)
	return ecdsa.SignASN1(rand.Reader, priv, digest[:])
}

// Verify returns nil if sig is a valid ASN.1 ECDSA-SHA256 signature of msg
// under pub. Accepts both Sign-produced and KMS-produced signatures.
func Verify(pub *ecdsa.PublicKey, msg, sig []byte) error {
	if pub == nil {
		return errors.New("nil public key")
	}
	digest := sha256.Sum256(msg)
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return errors.New("signature verification failed")
	}
	return nil
}

// EncodePublicKeyPEM serializes pub as a PEM block (PKIX SubjectPublicKeyInfo).
func EncodePublicKeyPEM(pub *ecdsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal pub: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// ParsePublicKeyPEM parses a PEM-encoded ECDSA P-256 public key.
func ParsePublicKeyPEM(pemBytes []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	return parseDERPublicKey(block.Bytes)
}

// parseDERPublicKey parses raw PKIX DER bytes (no PEM framing) into an ECDSA
// P-256 public key. Used by ParsePublicKeyPEM and by FetchKMSPublicKey, which
// receives DER directly from KMS.
func parseDERPublicKey(der []byte) (*ecdsa.PublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse pub: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected ECDSA public key, got %T", pub)
	}
	if ecPub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256 curve, got %s", ecPub.Curve.Params().Name)
	}
	return ecPub, nil
}

// EncodePrivateKeyPEM serializes priv as a PEM block (PKCS#8).
func EncodePrivateKeyPEM(priv *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal priv: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivateKeyPEM parses a PEM-encoded ECDSA P-256 private key.
func ParsePrivateKeyPEM(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse priv: %w", err)
	}
	ecPriv, ok := priv.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected ECDSA private key, got %T", priv)
	}
	if ecPriv.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256 curve, got %s", ecPriv.Curve.Params().Name)
	}
	return ecPriv, nil
}
