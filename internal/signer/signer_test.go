package signer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return priv
}

func TestSignVerifyRoundTrip(t *testing.T) {
	priv := newTestKey(t)
	msg := []byte("hello hint")

	sig, err := Sign(priv, msg)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := Verify(&priv.PublicKey, msg, sig); err != nil {
		t.Fatalf("expected verify ok, got %v", err)
	}
}

func TestVerifyTamperedMessageFails(t *testing.T) {
	priv := newTestKey(t)
	sig, err := Sign(priv, []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(&priv.PublicKey, []byte("tampered"), sig); err == nil {
		t.Fatal("expected verify to fail on tampered message, got nil")
	}
}

func TestVerifyTamperedSignatureFails(t *testing.T) {
	priv := newTestKey(t)
	sig, err := Sign(priv, []byte("msg"))
	if err != nil {
		t.Fatal(err)
	}
	sig[len(sig)-1] ^= 0xff

	if err := Verify(&priv.PublicKey, []byte("msg"), sig); err == nil {
		t.Fatal("expected verify to fail on tampered signature, got nil")
	}
}

func TestParsePublicKeyPEM(t *testing.T) {
	priv := newTestKey(t)
	pemBytes, err := EncodePublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	parsed, err := ParsePublicKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.Equal(&priv.PublicKey) {
		t.Fatal("round-trip mismatch")
	}
}

func TestParsePrivateKeyPEM(t *testing.T) {
	priv := newTestKey(t)
	pemBytes, err := EncodePrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	parsed, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.Equal(priv) {
		t.Fatal("round-trip mismatch")
	}
}
