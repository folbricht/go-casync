package desync

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalStoreCompressed(t *testing.T) {
	store := t.TempDir()

	s, err := NewLocalStore(store, StoreOptions{})
	require.NoError(t, err)

	// Make up some data and store it
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	// Store the chunk
	err = s.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store
	hasChunk, err := s.HasChunk(id)
	require.NoError(t, err)
	require.True(t, hasChunk, "chunk not found in store")

	// Pull the data the "official" way
	chunkOut, err := s.GetChunk(id)
	require.NoError(t, err)

	dataOut, err := chunkOut.Data()
	require.NoError(t, err)

	// Compare the data that went in with what came out
	require.Equal(t, dataIn, dataOut)

	// Now let's look at the file in the store directly to make sure it's compressed
	_, name := s.nameFromID(id)
	b, err := ioutil.ReadFile(name)
	require.NoError(t, err)
	require.NotEqual(t, dataIn, b, "chunk is not compressed")
}

func TestLocalStoreUncompressed(t *testing.T) {
	store := t.TempDir()

	s, err := NewLocalStore(store, StoreOptions{Uncompressed: true})
	require.NoError(t, err)

	// Make up some data and store it
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	err = s.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store
	hasChunk, err := s.HasChunk(id)
	require.NoError(t, err)
	require.True(t, hasChunk, "chunk not found in store")

	// Pull the data the "official" way
	chunkOut, err := s.GetChunk(id)
	require.NoError(t, err)

	dataOut, err := chunkOut.Data()
	require.NoError(t, err)

	// Compare the data that went in with what came out
	require.Equal(t, dataIn, dataOut)

	// Now let's look at the file in the store directly to make sure it's uncompressed
	_, name := s.nameFromID(id)
	b, err := ioutil.ReadFile(name)
	require.NoError(t, err)

	require.Equal(t, dataIn, b, "chunk is compressed")
}

func TestLocalStoreErrorHandling(t *testing.T) {
	store := t.TempDir()

	s, err := NewLocalStore(store, StoreOptions{})
	require.NoError(t, err)

	// Make up some data and store it
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()
	err = s.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Now put an invalid chunk into the store
	idInvalid, err := ChunkIDFromString("1000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)

	dirInvalid, nameInvalid := s.nameFromID(idInvalid)
	_ = os.Mkdir(dirInvalid, 0755)
	err = ioutil.WriteFile(nameInvalid, []byte("invalid data"), 0644)
	require.NoError(t, err)

	// Also add a blank chunk
	idBlank, err := ChunkIDFromString("2000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)

	dirBlank, nameBlank := s.nameFromID(idBlank)
	_ = os.Mkdir(dirBlank, 0755)
	err = ioutil.WriteFile(nameBlank, nil, 0644)
	require.NoError(t, err)

	// Let's see if we can retrieve the good chunk and get errors from the bad ones
	_, err = s.GetChunk(id)
	require.NoError(t, err)

	_, err = s.GetChunk(idInvalid)
	if _, ok := err.(ChunkInvalid); !ok {
		t.Fatal(err)
	}
	_, err = s.GetChunk(idBlank)
	if _, ok := err.(ChunkInvalid); !ok {
		t.Fatal(err)
	}

	// Run the verify with repair enabled which should get rid of the invalid and blank chunks
	err = s.Verify(context.Background(), 1, true, ioutil.Discard)
	require.NoError(t, err)

	// Let's see if we can still retrieve the good chunk and get Not Found for the others
	_, err = s.GetChunk(id)
	require.NoError(t, err)

	_, err = s.GetChunk(idInvalid)
	if _, ok := err.(ChunkMissing); !ok {
		t.Fatal(err)
	}
	_, err = s.GetChunk(idBlank)
	if _, ok := err.(ChunkMissing); !ok {
		t.Fatal(err)
	}
}

func TestLocalStoreUncompressedEncrypted(t *testing.T) {
	store := t.TempDir()

	s, err := NewLocalStore(store,
		StoreOptions{
			Uncompressed:       true,
			Encryption:         true,
			EncryptionPassword: "test-password",
		},
	)
	require.NoError(t, err)

	// Make up some data and store it
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	err = s.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store
	hasChunk, err := s.HasChunk(id)
	require.NoError(t, err)
	require.True(t, hasChunk, "chunk not found in store")

	// Pull the data the "official" way
	chunkOut, err := s.GetChunk(id)
	require.NoError(t, err)

	dataOut, err := chunkOut.Data()
	require.NoError(t, err)

	// Compare the data that went in with what came out
	require.Equal(t, dataIn, dataOut)

	// Now let's look at the file in the store directly to make sure it's actually
	// encrypted, meaning it should not match the plain (uncompressed) text
	_, name := s.nameFromID(id)
	b, err := ioutil.ReadFile(name)
	require.NoError(t, err)
	require.NotEqual(t, dataIn, b, "chunk is not encrypted")
}

func TestLocalStoreCompressedEncrypted(t *testing.T) {
	store := t.TempDir()

	s, err := NewLocalStore(store,
		StoreOptions{
			Uncompressed:       false,
			Encryption:         true,
			EncryptionPassword: "test-password",
		},
	)
	require.NoError(t, err)

	// Make up some data and store it
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	err = s.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store
	hasChunk, err := s.HasChunk(id)
	require.NoError(t, err)
	require.True(t, hasChunk, "chunk not found in store")

	// Pull the data the "official" way
	chunkOut, err := s.GetChunk(id)
	require.NoError(t, err)

	dataOut, err := chunkOut.Data()
	require.NoError(t, err)

	// Compare the data that went in with what came out
	require.Equal(t, dataIn, dataOut)

	// Now let's look at the file in the store directly and confirm it is
	// compressed and encrypted (in that order!).
	_, name := s.nameFromID(id)
	b, err := ioutil.ReadFile(name)
	require.NoError(t, err)

	// First decrypt it, using the correct password
	dec, _ := NewXChaCha20Poly1305("test-password")
	decrypted, err := dec.fromStorage(b)
	require.NoError(t, err)

	// Now decompress
	decompressed, err := Decompress(nil, decrypted)
	require.NoError(t, err)

	// And it should match the original content
	require.Equal(t, dataIn, decompressed)
}

func TestLocalStorePasswordMismatch(t *testing.T) {
	store := t.TempDir()

	// Build 2 stores accessing the same files but with different passwords
	s1, err := NewLocalStore(store,
		StoreOptions{
			Encryption:         true,
			EncryptionPassword: "good-password",
		},
	)
	require.NoError(t, err)
	s2, err := NewLocalStore(store,
		StoreOptions{
			Encryption:         true,
			EncryptionPassword: "bad-password",
		},
	)
	require.NoError(t, err)

	// Make up some data and store it using the good password
	dataIn := []byte("some data")

	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	err = s1.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Pull the data with the good password and compare it
	chunkOut, err := s1.GetChunk(id)
	require.NoError(t, err)
	dataOut, err := chunkOut.Data()
	require.NoError(t, err)
	require.Equal(t, dataIn, dataOut)

	// Try to get the chunk with a bad password, expect a not-found
	// since the chunk extensions are different for diff keys.
	_, err = s2.GetChunk(id)
	require.Error(t, err)

	if _, ok := err.(ChunkMissing); !ok {
		t.Fatalf("expected ChunkMissing error, but got %T", err)
	}
}
