package signer

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// SignWithKMS produces an ASN.1 DER-encoded ECDSA-SHA256 signature of msg by
// calling AWS KMS Sign on the key identified by keyID (key ID, ARN, or alias).
//
// The private key never leaves KMS. The IAM principal must have ***KMS-ACTION*** on
// that key. region overrides AWS_REGION when non-empty.
//
// The returned signature is bit-identical to what Sign() produces with a
// matching plaintext private key — same wire format, same verification path.
func SignWithKMS(ctx context.Context, keyID, region string, msg []byte) ([]byte, error) {
	digest := sha256.Sum256(msg)

	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	out, err := kms.NewFromConfig(cfg).Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          digest[:],
		MessageType:      types.MessageTypeDigest,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		return nil, fmt.Errorf("kms sign: %w", err)
	}
	return out.Signature, nil
}

// FetchKMSPublicKey returns the public key of a KMS asymmetric signing key as
// PEM-encoded PKIX bytes. Used during bootstrap to populate cmd/hint/pubkey.pem
// from the KMS key (so we don't have to handle a plaintext private key at all).
func FetchKMSPublicKey(ctx context.Context, keyID, region string) ([]byte, error) {
	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	out, err := kms.NewFromConfig(cfg).GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		return nil, fmt.Errorf("kms get-public-key: %w", err)
	}
	// out.PublicKey is already DER-encoded PKIX SubjectPublicKeyInfo.
	pub, err := parseDERPublicKey(out.PublicKey)
	if err != nil {
		return nil, err
	}
	return EncodePublicKeyPEM(pub)
}
