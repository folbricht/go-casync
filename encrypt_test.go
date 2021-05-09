package desync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	tests := map[string]struct {
		alg func(string) (converter, error)
	}{
		"xchacha20-poly1305": {func(pw string) (converter, error) { return NewXChaCha20Poly1305(pw) }},
		"aes-256-gcm":        {func(pw string) (converter, error) { return NewAES256GCM(pw) }},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			plainIn := []byte{1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

			// Make two converters. One for encryption and one for decryption. Could use
			// just one but this way we confirm the key generation is consistent
			enc, err := test.alg("secret-password")
			require.NoError(t, err)
			dec, err := test.alg("secret-password")
			require.NoError(t, err)

			// Encrypt the data
			ciphertext, err := enc.toStorage(plainIn)
			require.NoError(t, err)

			// Confirm the ciphertext is actually different than what went in
			require.NotEqual(t, plainIn, ciphertext)

			// Decrypt it
			plainOut, err := dec.fromStorage(ciphertext)
			require.NoError(t, err)

			// This should match the original data of course
			require.Equal(t, plainIn, plainOut)

			// Make another instance with a different password
			diffPw, err := test.alg("something-else")
			require.NoError(t, err)

			// Try to decrypt the data, should get an error from AEAD algorithms
			_, err = diffPw.fromStorage(ciphertext)
			require.Error(t, err)
		})
	}
}

func TestAES256GCMCompare(t *testing.T) {
	// Make three converters. Two with the same, one with a diff password
	enc1, err := NewAES256GCM("secret-password")
	require.NoError(t, err)
	enc2, err := NewAES256GCM("secret-password")
	require.NoError(t, err)
	diffPw, err := NewAES256GCM("something-else")
	require.NoError(t, err)

	// Check equality method
	require.True(t, enc1.equal(enc2))
	require.True(t, enc2.equal(enc1))
	require.False(t, diffPw.equal(enc1))
	require.False(t, enc1.equal(diffPw))
}

func TestAES256GCMExtension(t *testing.T) {
	enc1, err := NewAES256GCM("secret-password")
	require.NoError(t, err)

	// Confirm that we have a key-handle in the file extension
	require.Equal(t, ".aes-256-gcm-16db3403", enc1.extension)

	// If algorithm and password are the same, the same key
	// handle (extension) should be produced every time
	enc2, err := NewAES256GCM("secret-password")
	require.NoError(t, err)
	require.Equal(t, enc1.extension, enc2.extension)
}
