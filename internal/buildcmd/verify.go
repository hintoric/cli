package buildcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hintoric/cli/internal/plugins"
	"github.com/hintoric/cli/internal/signer"
)

func verifyCommand() *cobra.Command {
	var manifestPath, pubPath string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Validate manifest schema and ECDSA signature",
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := os.ReadFile(manifestPath)
			if err != nil {
				return err
			}
			pl, err := plugins.ParseManifest(body)
			if err != nil {
				return err
			}
			if err := plugins.ValidateManifest(pl); err != nil {
				return err
			}

			pubBytes, err := os.ReadFile(pubPath)
			if err != nil {
				return err
			}
			pub, err := signer.ParsePublicKeyPEM(pubBytes)
			if err != nil {
				return err
			}
			sig, err := os.ReadFile(manifestPath + ".sig")
			if err != nil {
				return err
			}
			if err := plugins.VerifyManifestSignature(pub, body, sig); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %d plugins, signature valid\n", len(pl.Plugins))
			return nil
		},
	}
	c.Flags().StringVar(&manifestPath, "manifest", "plugins.toml", "manifest path")
	c.Flags().StringVar(&pubPath, "pubkey", "pubkey.pem", "public key path")
	return c
}
