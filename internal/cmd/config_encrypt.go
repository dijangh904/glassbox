// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/glassbox/internal/config"
	"github.com/dotandev/glassbox/internal/errors"
	"github.com/spf13/cobra"
)

var (
	configEncryptInput  string
	configEncryptOutput string
	configDecryptInput  string
	configDecryptOutput string
)

// configEncryptCmd encrypts a plain-text config file.
var configEncryptCmd = &cobra.Command{
	Use:     "config:encrypt",
	GroupID: "utility",
	Short:   "Encrypt a Glassbox config file with AES-256-GCM",
	Long: `Encrypt a plain-text Glassbox config file so it can be stored securely.

The passphrase is read from --config-passphrase or GLASSBOX_CONFIG_PASSPHRASE.
The encrypted file can be loaded transparently by any Glassbox command that
reads config files when the same passphrase is supplied.

EXAMPLES
  glassbox config:encrypt --input .glassbox.toml --output .glassbox.enc.toml \
    --config-passphrase "my-secret"

  GLASSBOX_CONFIG_PASSPHRASE=my-secret \
    glassbox config:encrypt --input .glassbox.toml --output .glassbox.enc.toml`,
	Args: cobra.NoArgs,
	RunE: runConfigEncrypt,
}

// configDecryptCmd decrypts an encrypted config file to stdout or a file.
var configDecryptCmd = &cobra.Command{
	Use:     "config:decrypt",
	GroupID: "utility",
	Short:   "Decrypt an encrypted Glassbox config file",
	Long: `Decrypt a Glassbox config file that was encrypted with config:encrypt.

The passphrase is read from --config-passphrase or GLASSBOX_CONFIG_PASSPHRASE.
When --output is omitted the decrypted content is written to stdout.

EXAMPLES
  glassbox config:decrypt --input .glassbox.enc.toml \
    --config-passphrase "my-secret"

  glassbox config:decrypt --input .glassbox.enc.toml --output .glassbox.toml \
    --config-passphrase "my-secret"`,
	Args: cobra.NoArgs,
	RunE: runConfigDecrypt,
}

func init() {
	configEncryptCmd.Flags().StringVar(&configEncryptInput, "input", "", "Path to the plain-text config file (required)")
	configEncryptCmd.Flags().StringVar(&configEncryptOutput, "output", "", "Path to write the encrypted config file (required)")

	configDecryptCmd.Flags().StringVar(&configDecryptInput, "input", "", "Path to the encrypted config file (required)")
	configDecryptCmd.Flags().StringVar(&configDecryptOutput, "output", "", "Path to write the decrypted config (default: stdout)")

	rootCmd.AddCommand(configEncryptCmd)
	rootCmd.AddCommand(configDecryptCmd)
}

func runConfigEncrypt(cmd *cobra.Command, args []string) error {
	if configEncryptInput == "" {
		return errors.WrapCliArgumentRequired("input")
	}
	if configEncryptOutput == "" {
		return errors.WrapCliArgumentRequired("output")
	}

	passphrase := resolveConfigPassphrase()
	if passphrase == "" {
		return errors.WrapValidationError("passphrase is required: use --config-passphrase or GLASSBOX_CONFIG_PASSPHRASE")
	}

	plaintext, err := os.ReadFile(configEncryptInput)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to read input file: %v", err))
	}

	encrypted, err := config.EncryptConfig(plaintext, passphrase, configEncryptOutput)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("encryption failed: %v", err))
	}

	if err := os.WriteFile(configEncryptOutput, encrypted, 0600); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write encrypted file: %v", err))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Encrypted config written to %s\n", configEncryptOutput)
	return nil
}

func runConfigDecrypt(cmd *cobra.Command, args []string) error {
	if configDecryptInput == "" {
		return errors.WrapCliArgumentRequired("input")
	}

	passphrase := resolveConfigPassphrase()
	if passphrase == "" {
		return errors.WrapValidationError("passphrase is required: use --config-passphrase or GLASSBOX_CONFIG_PASSPHRASE")
	}

	plaintext, err := config.LoadEncryptedConfig(configDecryptInput, passphrase, nil)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("decryption failed: %v", err))
	}

	if configDecryptOutput == "" {
		fmt.Fprint(cmd.OutOrStdout(), string(plaintext))
		return nil
	}

	if err := os.WriteFile(configDecryptOutput, plaintext, 0600); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write decrypted file: %v", err))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Decrypted config written to %s\n", configDecryptOutput)
	return nil
}

// resolveConfigPassphrase returns the passphrase from the CLI flag or env var.
func resolveConfigPassphrase() string {
	if ConfigPassphraseFlag != "" {
		return ConfigPassphraseFlag
	}
	return os.Getenv("GLASSBOX_CONFIG_PASSPHRASE")
}
