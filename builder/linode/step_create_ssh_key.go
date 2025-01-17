package linode

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

// StepCreateSSHKey represents a Packer build step that generates SSH key pairs.
type StepCreateSSHKey struct {
	Debug        bool
	DebugKeyPath string
}

// Run executes the Packer build step that generates SSH key pairs.
// The key pairs are added to the ssh config
func (s *StepCreateSSHKey) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	handleError := func(prefix string, err error) multistep.StepAction {
		return errorHelper(state, ui, prefix, err)
	}

	if config.Comm.SSHPrivateKeyFile != "" {
		ui.Say("Using existing SSH private key")
		privateKeyBytes, err := os.ReadFile(config.Comm.SSHPrivateKeyFile)
		if err != nil {
			return handleError("Error loading configured private key file", err)
		}

		config.Comm.SSHPrivateKey = privateKeyBytes
		config.Comm.SSHPublicKey = nil

		return multistep.ActionContinue
	}

	ui.Say("Creating temporary SSH key for instance...")
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return handleError("Error creating temporary ssh key", err)
	}

	priv_blk := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(priv),
	}

	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return handleError("Error creating temporary ssh key", err)
	}
	config.Comm.SSHPrivateKey = pem.EncodeToMemory(&priv_blk)
	config.Comm.SSHPublicKey = ssh.MarshalAuthorizedKey(pub)

	// Linode has a serious issue with the newline that the ssh package appends to the end of the key.
	if config.Comm.SSHPublicKey[len(config.Comm.SSHPublicKey)-1] == '\n' {
		config.Comm.SSHPublicKey = config.Comm.SSHPublicKey[:len(config.Comm.SSHPublicKey)-1]
	}

	if s.Debug {
		ui.Message(fmt.Sprintf("Saving key for debug purposes: %s", s.DebugKeyPath))
		f, err := os.Create(s.DebugKeyPath)
		if err != nil {
			return handleError("Error saving debug key", err)
		}

		// Write out the key
		err = pem.Encode(f, &priv_blk)
		f.Close()
		if err != nil {
			return handleError("Error saving debug key", err)
		}
	}
	return multistep.ActionContinue
}

// Nothing to clean up. SSH keys are associated with a single Linode instance.
func (s *StepCreateSSHKey) Cleanup(state multistep.StateBag) {}
