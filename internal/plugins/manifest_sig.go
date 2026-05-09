package plugins

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/hintoric/cli/internal/signer"
)

// VerifyManifestSignature returns nil iff sig is a valid ECDSA-SHA256
// (ASN.1 DER) signature of manifestBody under pub.
func VerifyManifestSignature(pub *ecdsa.PublicKey, manifestBody, sig []byte) error {
	if err := signer.Verify(pub, manifestBody, sig); err != nil {
		return fmt.Errorf("manifest signature: %w", err)
	}
	return nil
}
