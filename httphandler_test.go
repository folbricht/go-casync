package desync

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPHandlerReadWrite(t *testing.T) {
	store := t.TempDir()

	upstream, err := NewLocalStore(store, StoreOptions{})
	require.NoError(t, err)

	// Start a read-write capable server and a read-only server
	rw := httptest.NewServer(NewHTTPHandler(upstream, true, false, []converter{Compressor{}}, ""))
	defer rw.Close()
	ro := httptest.NewServer(NewHTTPHandler(upstream, false, false, []converter{Compressor{}}, ""))
	defer ro.Close()

	// Initialize HTTP chunks stores, one RW and the other RO
	rwStoreURL, _ := url.Parse(rw.URL)
	rwStore, err := NewRemoteHTTPStore(rwStoreURL, StoreOptions{})
	require.NoError(t, err)

	roStoreURL, _ := url.Parse(ro.URL)
	roStore, err := NewRemoteHTTPStore(roStoreURL, StoreOptions{})
	require.NoError(t, err)

	// Make up some data and store it in the RW store
	dataIn := []byte("some data")
	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	// Write a chunk
	err = rwStore.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store
	hasChunk, err := rwStore.HasChunk(id)
	require.NoError(t, err)
	require.True(t, hasChunk)

	// Let's try to send some data to the RO store, that should fail
	err = roStore.StoreChunk(chunkIn)
	require.Error(t, err, "expected error writing to read-only chunkstore")
}

func TestHTTPHandlerCompression(t *testing.T) {
	store := t.TempDir()

	upstream, err := NewLocalStore(store, StoreOptions{})
	require.NoError(t, err)

	// Start a server that uses compression, and one that serves uncompressed chunks
	co := httptest.NewServer(NewHTTPHandler(upstream, true, false, []converter{Compressor{}}, ""))
	defer co.Close()
	un := httptest.NewServer(NewHTTPHandler(upstream, true, false, nil, ""))
	defer un.Close()

	// Initialize HTTP chunks stores, one RW and the other RO. Also make one that's
	// trying to get compressed data from a HTTP store that serves only uncompressed.
	coStoreURL, _ := url.Parse(co.URL)
	coStore, err := NewRemoteHTTPStore(coStoreURL, StoreOptions{})
	require.NoError(t, err)

	unStoreURL, _ := url.Parse(un.URL)
	unStore, err := NewRemoteHTTPStore(unStoreURL, StoreOptions{Uncompressed: true})
	require.NoError(t, err)

	invalidStore, err := NewRemoteHTTPStore(unStoreURL, StoreOptions{})
	require.NoError(t, err)

	// Make up some data and store it in the RW store
	dataIn := []byte("some data")
	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	// Try to get compressed chunks from a store that only serves uncompressed chunks
	_, err = invalidStore.GetChunk(id)
	require.Error(t, err, "expected failure trying to get compressed chunks from uncompressed http store")

	err = coStore.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Check it's in the store when looking for compressed chunks
	coExists, err := coStore.HasChunk(id)
	require.NoError(t, err)
	require.True(t, coExists)

	// It's also visible when looking for uncompressed data
	unExists, err := unStore.HasChunk(id)
	require.NoError(t, err)
	require.True(t, unExists)

	// Send it uncompressed
	err = unStore.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Try to get the uncompressed chunk
	_, err = unStore.GetChunk(id)
	require.NoError(t, err)
}

func TestHTTPHandlerEncryption(t *testing.T) {
	// Prep a local store (no encryption)
	store := t.TempDir()
	upstream, err := NewLocalStore(store, StoreOptions{})
	require.NoError(t, err)

	// Start a read-write capable server with Encryption, no Compression
	enc, err := NewXChaCha20Poly1305("testpassword")
	require.NoError(t, err)
	server := httptest.NewServer(NewHTTPHandler(upstream, true, false, []converter{enc}, ""))
	defer server.Close()

	// Initialize HTTP chunks store (client)
	httpStoreURL, _ := url.Parse(server.URL)
	httpStore, err := NewRemoteHTTPStore(httpStoreURL, StoreOptions{
		Uncompressed:       true,
		Encryption:         true,
		EncryptionPassword: "testpassword",
	})
	require.NoError(t, err)

	// Make up some data and store it in the RW store
	dataIn := []byte("some data")
	chunkIn := NewChunk(dataIn)
	id := chunkIn.ID()

	// Write a chunk via HTTP
	err = httpStore.StoreChunk(chunkIn)
	require.NoError(t, err)

	// Read it back via HTTP and compare to the original
	chunkOut, err := httpStore.GetChunk(id)
	require.NoError(t, err)
	dataOut, err := chunkOut.Data()
	require.NoError(t, err)
	require.Equal(t, dataIn, dataOut)
}
