package desync

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// xchacha20poly1305 is an encryption layer for chunk storage. It
// encrypts/decrypts to/from storage using ChaCha20-Poly1305 AEAD.
// The key is generated from a passphrase with SHA256.
type xchacha20poly1305 struct {
	key  []byte
	aead cipher.AEAD

	// Chunk extension with identifier derived from the key.
	extension string
}

var _ converter = xchacha20poly1305{}

func NewXChaCha20Poly1305(passphrase string) (xchacha20poly1305, error) {
	key := sha256.Sum256([]byte(passphrase))
	keyHash := sha256.Sum256(key[:])
	extension := fmt.Sprintf(".xchacha20-poly1305-%x", keyHash[:4])
	aead, err := chacha20poly1305.NewX(key[:])
	return xchacha20poly1305{key: key[:], aead: aead, extension: extension}, err
}

// encrypt for storage. The nonce is prepended to the data.
func (d xchacha20poly1305) toStorage(in []byte) ([]byte, error) {
	out := make([]byte, d.aead.NonceSize(), d.aead.NonceSize()+len(in)+d.aead.Overhead())
	nonce := out[:d.aead.NonceSize()]
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return d.aead.Seal(out, nonce, in, nil), nil
}

// decrypt from storage. The nonce is taken from the start of the
// chunk data. This by itself does not verify integrity. That
// is achieved by the existing chunk validation.
func (d xchacha20poly1305) fromStorage(in []byte) ([]byte, error) {
	if len(in) < d.aead.NonceSize() {
		return nil, errors.New("no nonce prefix found in chunk, not encrypted or wrong algorithm")
	}
	nonce := in[:d.aead.NonceSize()]
	return d.aead.Open(nil, nonce, in[d.aead.NonceSize():], nil)
}

func (d xchacha20poly1305) equal(c converter) bool {
	other, ok := c.(xchacha20poly1305)
	if !ok {
		return false
	}
	return bytes.Equal(d.key, other.key)
}

func (d xchacha20poly1305) storageExtension() string {
	return d.extension
}

// aes256gcm is an encryption layer for chunk storage. It
// encrypts/decrypts to/from storage using AES 256 GCM.
// The key is generated from a passphrase with SHA256.
type aes256gcm struct {
	key  []byte
	aead cipher.AEAD

	// Chunk extension with identifier derived from the key.
	extension string
}

var _ converter = aes256gcm{}

func NewAES256GCM(passphrase string) (aes256gcm, error) {
	key := sha256.Sum256([]byte(passphrase))
	keyHash := sha256.Sum256(key[:])
	extension := fmt.Sprintf(".aes-256-gcm-%x", keyHash[:4])
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return aes256gcm{}, err
	}
	aead, err := cipher.NewGCM(block)
	return aes256gcm{key: key[:], aead: aead, extension: extension}, err
}

// encrypt for storage. The nonce is prepended to the data.
func (d aes256gcm) toStorage(in []byte) ([]byte, error) {
	out := make([]byte, d.aead.NonceSize(), d.aead.NonceSize()+len(in)+d.aead.Overhead())
	nonce := out[:d.aead.NonceSize()]
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return d.aead.Seal(out, nonce, in, nil), nil
}

// decrypt from storage. The nonce is taken from the start of the
// chunk data. This by itself does not verify integrity. That
// is achieved by the existing chunk validation.
func (d aes256gcm) fromStorage(in []byte) ([]byte, error) {
	if len(in) < d.aead.NonceSize() {
		return nil, errors.New("no nonce prefix found in chunk, not encrypted or wrong algorithm")
	}
	nonce := in[:d.aead.NonceSize()]
	return d.aead.Open(nil, nonce, in[d.aead.NonceSize():], nil)
}

func (d aes256gcm) equal(c converter) bool {
	other, ok := c.(aes256gcm)
	if !ok {
		return false
	}
	return bytes.Equal(d.key, other.key)
}

func (d aes256gcm) storageExtension() string {
	return d.extension
}
