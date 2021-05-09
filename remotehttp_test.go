package desync

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPStoreURL(t *testing.T) {
	var requestURI string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	chunkID := ChunkID{1, 2, 3, 4}
	tests := map[string]struct {
		storePath  string
		serverPath string
	}{
		"no path":             {"", "/0102/0102030400000000000000000000000000000000000000000000000000000000.cacnk"},
		"slash only":          {"/", "/0102/0102030400000000000000000000000000000000000000000000000000000000.cacnk"},
		"no trailing slash":   {"/path", "/path/0102/0102030400000000000000000000000000000000000000000000000000000000.cacnk"},
		"with trailing slash": {"/path/", "/path/0102/0102030400000000000000000000000000000000000000000000000000000000.cacnk"},
		"long path":           {"/path1/path2", "/path1/path2/0102/0102030400000000000000000000000000000000000000000000000000000000.cacnk"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			u.Path = test.storePath
			s, err := NewRemoteHTTPStore(u, StoreOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s.GetChunk(chunkID)
			if requestURI != test.serverPath {
				t.Fatalf("got request uri '%s', want '%s'", requestURI, test.serverPath)
			}
		})
	}
}

func TestHasChunk(t *testing.T) {
	var attemptCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		switch r.URL.String() {
		case "/0000/0000000100000000000000000000000000000000000000000000000000000000.cacnk":
			w.WriteHeader(http.StatusOK)
		case "/0000/0000000200000000000000000000000000000000000000000000000000000000.cacnk":
			w.WriteHeader(http.StatusNotFound)
		case "/0000/0000000300000000000000000000000000000000000000000000000000000000.cacnk":
			w.WriteHeader(http.StatusBadRequest)
		case "/0000/0000000400000000000000000000000000000000000000000000000000000000.cacnk":
			w.WriteHeader(http.StatusForbidden)
		case "/0000/0000000500000000000000000000000000000000000000000000000000000000.cacnk":
			w.WriteHeader(http.StatusBadGateway)
			io.WriteString(w, "Bad Gateway")
		case "/0000/0000000600000000000000000000000000000000000000000000000000000000.cacnk":
			if attemptCount >= 2 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, "Bad Gateway")
			}
		case "/0000/0000000700000000000000000000000000000000000000000000000000000000.cacnk":
			if attemptCount >= 3 {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, "Bad Gateway")
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	tests := map[string]struct {
		chunkId      ChunkID
		hasChunk     bool
		hasError     bool
		attemptCount int
	}{
		// The default case is a successful chunk test operation
		"chunk exists": {ChunkID{0, 0, 0, 1}, true, false, 1},
		// HTTP 404 Not Found - Testing a chunk that does not exist should result in an immediate 'does not exist' response
		"chunk does not exist": {ChunkID{0, 0, 0, 2}, false, false, 1},
		// HTTP 400 Bad Request - should fail immediately
		"bad request": {ChunkID{0, 0, 0, 3}, false, true, 1},
		// HTTP 403 Forbidden - should fail immediately
		"forbidden": {ChunkID{0, 0, 0, 4}, false, true, 1},
		// HTTP 503 Bad Gateway - should retry, but ultimately fail
		"permanent 503": {ChunkID{0, 0, 0, 5}, false, true, 5},
		// HTTP 503 Bad Gateway - should retry, and a subsequent successful call should return that the chunk exists
		"temporary 503, then chunk exists": {ChunkID{0, 0, 0, 6}, true, false, 2},
		// HTTP 503 Bad Gateway - should retry, and a subsequent successful call should report that the chunk does not exist immediately
		"temporary 503, then chunk does not exist": {ChunkID{0, 0, 0, 7}, false, false, 3},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			u.Path = "/"
			s, err := NewRemoteHTTPStore(u, StoreOptions{ErrorRetry: 5, ErrorRetryBaseInterval: time.Microsecond})
			if err != nil {
				t.Fatal(err)
			}

			attemptCount = 0
			hasChunk, err := s.HasChunk(test.chunkId)
			if (hasChunk != test.hasChunk) || ((err != nil) != test.hasError) || (attemptCount != test.attemptCount) {
				t.Errorf("expected hasChunk = %t / hasError = %t / attemptCount = %d, got %t / %t / %d", test.hasChunk, test.hasError, test.attemptCount, hasChunk, (err != nil), attemptCount)
			}
		})
	}
}

func TestGetChunk(t *testing.T) {
	var attemptCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		switch r.URL.String() {
		case "/3bc8/3bc8e3230df5515b1b40e938e49ebc765c6157d4cf4e2b9d5f9c272571365395":
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "Chunk Content String 1")
		case "/0000/0000000100000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "Chunk Content With hash mismatch")
		case "/0000/0000000200000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusNotFound)
		case "/0000/0000000300000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "BadRequest")
		case "/0000/0000000400000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Forbidden")
		case "/0000/0000000500000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusBadGateway)
			io.WriteString(w, "Bad Gateway")
		case "/65a1/65a128d0658c4cf0941771c7090fea6d9c6f981810659c24c91ba23edd71574b":
			if attemptCount >= 2 {
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, "Chunk Content String 6")
			} else {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, "Bad Gateway")
			}
		case "/0000/0000000700000000000000000000000000000000000000000000000000000000":
			if attemptCount >= 3 {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, "Bad Gateway")
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	tests := map[string]struct {
		chunkId      ChunkID
		content      string
		hasError     bool
		attemptCount int
	}{

		// The default case is a successful get chunk operation
		"chunk exists": {ChunkID{0x3b, 0xc8, 0xe3, 0x23, 0x0d, 0xf5, 0x51, 0x5b, 0x1b, 0x40, 0xe9, 0x38, 0xe4, 0x9e, 0xbc, 0x76, 0x5c, 0x61, 0x57, 0xd4, 0xcf, 0x4e, 0x2b, 0x9d, 0x5f, 0x9c, 0x27, 0x25, 0x71, 0x36, 0x53, 0x95}, "Chunk Content String 1", false, 1},
		// Fetching a chunk where the hash does not match the contents should fail for a store where verification is enabled
		"chunk exists, but invalid hash": {ChunkID{0, 0, 0, 1}, "", true, 1},
		// HTTP 404 Not Found - Fetching a chunk that does not exist should fail immediately
		"chunk does not exist": {ChunkID{0, 0, 0, 2}, "", true, 1},
		// HTTP 400 Bad Request - should fail immediately
		"bad request": {ChunkID{0, 0, 0, 3}, "", true, 1},
		// HTTP 403 Forbidden - should fail immediately
		"forbidden": {ChunkID{0, 0, 0, 4}, "", true, 1},
		// HTTP 503 Bad Gateway - should retry, but ultimately fail
		"permanent 503": {ChunkID{0, 0, 0, 5}, "", true, 5},
		// HTTP 503 Bad Gateway - should retry, and a subsequent successful call should return a successful chunk
		"temporary 503, then chunk exists": {ChunkID{0x65, 0xa1, 0x28, 0xd0, 0x65, 0x8c, 0x4c, 0xf0, 0x94, 0x17, 0x71, 0xc7, 0x09, 0x0f, 0xea, 0x6d, 0x9c, 0x6f, 0x98, 0x18, 0x10, 0x65, 0x9c, 0x24, 0xc9, 0x1b, 0xa2, 0x3e, 0xdd, 0x71, 0x57, 0x4b}, "Chunk Content String 6", false, 2},
		// HTTP 503 Bad Gateway - should retry, and a subsequent successful call should report that the chunk does not exist, thereby failing immediately
		"temporary 503, then chunk does not exist": {ChunkID{0, 0, 0, 7}, "", true, 3},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			u.Path = "/"
			s, err := NewRemoteHTTPStore(u, StoreOptions{ErrorRetry: 5, ErrorRetryBaseInterval: time.Microsecond, Uncompressed: true})
			if err != nil {
				t.Fatal(err)
			}

			attemptCount = 0
			content, err := s.GetChunk(test.chunkId)
			content_string := ""
			if content != nil {
				uncompressedContent, _ := content.Data()
				content_string = string(uncompressedContent)
			}
			if (content_string != test.content) || ((err != nil) != test.hasError) || (attemptCount != test.attemptCount) {
				t.Errorf("expected content = \"%s\" / hasError = %t / attemptCount = %d, got \"%s\" / %t / %d", test.content, test.hasError, test.attemptCount, content_string, (err != nil), attemptCount)
			}
		})
	}
}

func TestPutChunk(t *testing.T) {
	var attemptCount int
	var writtenContent []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		switch r.URL.String() {
		case "/3bc8/3bc8e3230df5515b1b40e938e49ebc765c6157d4cf4e2b9d5f9c272571365395":
			content, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, err.Error())
			} else {
				writtenContent = content
				w.WriteHeader(http.StatusOK)
			}
		case "/0000/0000000300000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "BadRequest")
		case "/0000/0000000400000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Forbidden")
		case "/0000/0000000500000000000000000000000000000000000000000000000000000000":
			w.WriteHeader(http.StatusBadGateway)
			io.WriteString(w, "Bad Gateway")
		case "/65a1/65a128d0658c4cf0941771c7090fea6d9c6f981810659c24c91ba23edd71574b":
			if attemptCount >= 2 {
				content, err := ioutil.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					io.WriteString(w, err.Error())
				} else {
					writtenContent = content
					w.WriteHeader(http.StatusOK)
				}
			} else {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, "Bad Gateway")
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	tests := map[string]struct {
		chunkId        ChunkID
		content        string
		writtenContent string
		hasError       bool
		attemptCount   int
	}{
		// The typical path is a successful store operation
		"store chunk successful": {ChunkID{0x3b, 0xc8, 0xe3, 0x23, 0x0d, 0xf5, 0x51, 0x5b, 0x1b, 0x40, 0xe9, 0x38, 0xe4, 0x9e, 0xbc, 0x76, 0x5c, 0x61, 0x57, 0xd4, 0xcf, 0x4e, 0x2b, 0x9d, 0x5f, 0x9c, 0x27, 0x25, 0x71, 0x36, 0x53, 0x95}, "Chunk Content String 1", "Chunk Content String 1", false, 1},
		// Attempting to store a chunk with null content will be errored by the library itself, and will not result in any HTTP requests
		"store chunk not allowed with no chunk content": {ChunkID{0, 0, 0, 2}, "", "", true, 0},
		// HTTP 400 Bad Request - should fail immediately
		"bad request": {ChunkID{0, 0, 0, 3}, "3", "", true, 1},
		// HTTP 403 Forbidden - should fail immediately
		"forbidden": {ChunkID{0, 0, 0, 4}, "4", "", true, 1},
		// HTTP 503 Bad Gateway - should retry, but ultimately fail
		"permanent 503": {ChunkID{0, 0, 0, 5}, "5", "", true, 5},
		// HTTP 503 Bad Gateway - should retry, and a subsequent successful call should make the entire operation succeed
		"temporary 503, then store chunk successful": {ChunkID{0x65, 0xa1, 0x28, 0xd0, 0x65, 0x8c, 0x4c, 0xf0, 0x94, 0x17, 0x71, 0xc7, 0x09, 0x0f, 0xea, 0x6d, 0x9c, 0x6f, 0x98, 0x18, 0x10, 0x65, 0x9c, 0x24, 0xc9, 0x1b, 0xa2, 0x3e, 0xdd, 0x71, 0x57, 0x4b}, "Chunk Content String 6", "Chunk Content String 6", false, 2},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			u.Path = "/"
			s, err := NewRemoteHTTPStore(u, StoreOptions{ErrorRetry: 5, ErrorRetryBaseInterval: time.Microsecond, Uncompressed: true})
			if err != nil {
				t.Fatal(err)
			}

			attemptCount = 0
			writtenContent = nil
			chunk, _ := NewChunkWithID(test.chunkId, []byte(test.content), true)
			err = s.StoreChunk(chunk)
			writtenContentString := ""
			if writtenContent != nil {
				writtenContentString = string(writtenContent)
			}
			if ((err != nil) != test.hasError) || (attemptCount != test.attemptCount) || (writtenContentString != test.writtenContent) {
				t.Errorf("expected writtenContent = \"%s\" / hasError = %t / attemptCount = %d, got \"%s\" / %t / %d", test.writtenContent, test.hasError, test.attemptCount, writtenContentString, (err != nil), attemptCount)
			}
		})
	}
}

func TestRemoteHTTPPutEncrypted(t *testing.T) {
	body := new(bytes.Buffer)

	// Setup a dummy server that records the request body (raw chunk data)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(body, r.Body)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	// HTTP client store with encryption and compression
	httpStore, err := NewRemoteHTTPStore(u, StoreOptions{
		Uncompressed:       false,
		Encryption:         true,
		EncryptionPassword: "testpassword",
	})
	require.NoError(t, err)

	// Prep a test chunk
	dataIn := []byte("some data")
	chunkIn := NewChunk(dataIn)

	// Send the chunk over HTTP
	err = httpStore.StoreChunk(chunkIn)
	require.NoError(t, err)

	// If everything worked, the request body should be the chunk data, first
	// compressed, then encrypted. Unwind it manually to check the layers are in order.
	dec, err := NewXChaCha20Poly1305("testpassword")
	require.NoError(t, err)
	decrypted, err := dec.fromStorage(body.Bytes())
	require.NoError(t, err)
	uncompressed, err := Decompress(nil, decrypted)
	require.NoError(t, err)
	require.Equal(t, dataIn, uncompressed)
}
