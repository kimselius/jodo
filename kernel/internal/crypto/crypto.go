package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

// Encryptor provides AES-256-GCM encryption/decryption for storing secrets in the database.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewFromEnv creates an Encryptor using the ENCRYPTION_KEY environment variable.
func NewFromEnv() (*Encryptor, error) {
	keyB64 := os.Getenv("ENCRYPTION_KEY")
	if keyB64 == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable not set")
	}
	return New(keyB64)
}

// New creates an Encryptor from a base64-encoded 32-byte key.
func New(keyB64 string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (got %d)", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext and returns nonce+ciphertext as a single byte slice.
func (e *Encryptor) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return e.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decrypts nonce+ciphertext produced by Encrypt.
func (e *Encryptor) Decrypt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// SSHKeyPair holds a generated Ed25519 SSH keypair.
type SSHKeyPair struct {
	PrivateKeyPEM string // PEM-encoded private key
	PublicKeySSH  string // OpenSSH authorized_keys format
}

// GenerateSSHKey generates a new Ed25519 SSH keypair.
func GenerateSSHKey() (*SSHKeyPair, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Encode private key as PEM (OpenSSH format via x/crypto/ssh)
	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privPEMBytes := pem.EncodeToMemory(privPEM)

	// Encode public key in OpenSSH authorized_keys format
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("create ssh public key: %w", err)
	}
	pubSSH := string(ssh.MarshalAuthorizedKey(sshPub))

	return &SSHKeyPair{
		PrivateKeyPEM: string(privPEMBytes),
		PublicKeySSH:  pubSSH,
	}, nil
}
