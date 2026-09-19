// Package kms is the Keeper a deployment uses: Cloud KMS.
//
// The wrapping key lives in Google's key management service and never leaves
// it. This process sends a data key and receives it wrapped, and sends it
// wrapped and receives it back — it cannot read the key that did either, and
// neither can anyone who reads this database. That is what makes destroying a
// wrapped key final (docs/requirements.md, section 21).
package kms

import (
	"context"
	"fmt"

	kmsapi "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

// Name is what a key wrapped by this keeper is recorded as.
const Name = "cloudkms"

// Keeper wraps data keys with a Cloud KMS key.
type Keeper struct {
	client *kmsapi.KeyManagementClient
	// key is the full resource name of the key version's parent key, as
	// Terraform writes it (infra/terraform/audit.tf).
	key string
}

// New returns a keeper using the named key.
func New(ctx context.Context, key string) (*Keeper, error) {
	if key == "" {
		return nil, fmt.Errorf("the key manager needs the name of a key")
	}

	client, err := kmsapi.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the key manager: %w", err)
	}
	return &Keeper{client: client, key: key}, nil
}

// Name reports which keeper this is.
func (k *Keeper) Name() string { return Name }

// Wrap asks the key manager to encrypt a data key.
func (k *Keeper) Wrap(ctx context.Context, key []byte) ([]byte, error) {
	answer, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      k.key,
		Plaintext: key,
	})
	if err != nil {
		return nil, fmt.Errorf("the key manager did not wrap the key: %w", err)
	}
	return answer.GetCiphertext(), nil
}

// Unwrap asks it to decrypt one.
func (k *Keeper) Unwrap(ctx context.Context, wrapped []byte) ([]byte, error) {
	answer, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       k.key,
		Ciphertext: wrapped,
	})
	if err != nil {
		return nil, fmt.Errorf("the key manager did not unwrap the key: %w", err)
	}
	return answer.GetPlaintext(), nil
}

// Close releases the client.
func (k *Keeper) Close() error { return k.client.Close() }
