package buildcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/signer"
)

func signCommand() *cobra.Command {
	var manifestPath, keyPath, kmsKeyID, kmsRegion string
	c := &cobra.Command{
		Use:   "sign",
		Short: "Produce <manifest>.sig with an ECDSA P-256 key",
		Long: `Sign the plugin manifest with an ECDSA P-256 key.

Exactly one of these must be supplied:
  --key <path>            Plain PEM-encoded PKCS#8 ECDSA P-256 private key.
                          Use this for local development.
  --kms-key-id <id>       AWS KMS asymmetric signing key (key id, ARN, or
                          alias such as ***KMS-ALIAS***). The
                          private key never leaves KMS; signing happens via
                          ***KMS-ACTION*** on the current AWS principal. Use this
                          in CI / production releases.

The output signature is ASN.1 DER ECDSA-SHA256, identical wire format
whether produced locally or via KMS.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (keyPath == "") == (kmsKeyID == "") {
				return fmt.Errorf("specify exactly one of --key or --kms-key-id")
			}

			body, err := os.ReadFile(manifestPath)
			if err != nil {
				return err
			}

			var sig []byte
			switch {
			case keyPath != "":
				pemBytes, err := os.ReadFile(keyPath)
				if err != nil {
					return err
				}
				priv, err := signer.ParsePrivateKeyPEM(pemBytes)
				if err != nil {
					return err
				}
				sig, err = signer.Sign(priv, body)
				if err != nil {
					return err
				}
			case kmsKeyID != "":
				sig, err = signer.SignWithKMS(cmd.Context(), kmsKeyID, kmsRegion, body)
				if err != nil {
					return err
				}
			}

			if err := os.WriteFile(manifestPath+".sig", sig, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "signed %s → %s.sig (%d bytes)\n", manifestPath, manifestPath, len(sig))
			return nil
		},
	}
	c.Flags().StringVar(&manifestPath, "manifest", "plugins.toml", "manifest path")
	c.Flags().StringVar(&keyPath, "key", "", "plain PEM ECDSA P-256 private key (PKCS#8)")
	c.Flags().StringVar(&kmsKeyID, "kms-key-id", "", "AWS KMS key id, ARN, or alias")
	c.Flags().StringVar(&kmsRegion, "kms-region", "", "AWS region (overrides AWS_REGION)")
	return c
}
