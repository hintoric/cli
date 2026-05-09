// Command genkey generates a fresh ECDSA P-256 keypair for local hint development.
//
// It writes the public key to cmd/hint/pubkey.pem (committed and embedded into
// the host binary) and the private key to keys/hint-dev.key (gitignored).
//
// For production releases the private key lives in AWS KMS and is never
// exported; use `aws kms get-public-key` to fetch the corresponding pubkey
// directly from KMS instead of running this tool.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/hintoric/cli/internal/signer"
)

func main() {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	pubPEM, err := signer.EncodePublicKeyPEM(&priv.PublicKey)
	if err != nil {
		panic(err)
	}
	privPEM, err := signer.EncodePrivateKeyPEM(priv)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll("keys", 0o755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll("cmd/hint", 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("cmd/hint/pubkey.pem", pubPEM, 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("keys/hint-dev.key", privPEM, 0o600); err != nil {
		panic(err)
	}
	fmt.Println("wrote cmd/hint/pubkey.pem and keys/hint-dev.key")
}
