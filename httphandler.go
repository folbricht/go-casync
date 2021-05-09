package desync

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// HTTPHandler is the server-side handler for a HTTP chunk store.
type HTTPHandler struct {
	HTTPHandlerBase
	s               Store
	SkipVerifyWrite bool

	// Storage-side of the converters in this case is towards the client
	converters Converters

	// Use the file extension for compressed chunks
	compressed bool
}

// NewHTTPHandler initializes and returns a new HTTP handler for a chunks server.
func NewHTTPHandler(s Store, writable, skipVerifyWrite bool, converters Converters, auth string) http.Handler {
	compressed := converters.hasCompression()
	return HTTPHandler{HTTPHandlerBase{"chunk", writable, auth}, s, skipVerifyWrite, converters, compressed}
}

func (h HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.authorization != "" && r.Header.Get("Authorization") != h.authorization {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := h.idFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Method {
	case "GET":
		h.get(id, w)
	case "HEAD":
		h.head(id, w)
	case "PUT":
		h.put(id, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("only GET, PUT and HEAD are supported"))
	}
}

func (h HTTPHandler) get(id ChunkID, w http.ResponseWriter) {
	var b []byte
	chunk, err := h.s.GetChunk(id)
	if err == nil {
		// Optimization for when the chunk modifiers match those
		// of the chunk server. In that case it's not necessary
		// to convert back and forth. Just use the raw data as loaded
		// from the store.
		if len(chunk.storage) > 0 && h.converters.equal(chunk.converters) {
			b = chunk.storage
		} else {
			b, err = chunk.Data()
			if err == nil {
				b, err = h.converters.toStorage(b)
			}
		}
	}
	h.HTTPHandlerBase.get(id.String(), b, err, w)
}

func (h HTTPHandler) head(id ChunkID, w http.ResponseWriter) {
	hasChunk, err := h.s.HasChunk(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hasChunk {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (h HTTPHandler) put(id ChunkID, w http.ResponseWriter, r *http.Request) {
	err := h.HTTPHandlerBase.validateWritable(h.s.String(), w, r)
	if err != nil {
		return
	}

	// The upstream store needs to support writing as well
	s, ok := h.s.(WriteStore)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "upstream chunk store '%s' does not support writing\n", h.s)
		return
	}

	// Read the raw chunk data into memory
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}

	// Turn it into a chunk, and validate the ID unless verification is disabled
	chunk, err := NewChunkFromStorage(id, b.Bytes(), h.converters, h.SkipVerifyWrite)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Store it upstream
	if err := s.StoreChunk(chunk); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h HTTPHandler) idFromPath(p string) (ChunkID, error) {
	ext := h.converters.storageExtension()
	if !strings.HasSuffix(p, ext) {
		return ChunkID{}, errors.New("invalid chunk type, verify compression and encryption settings")
	}
	sID := strings.TrimSuffix(path.Base(p), ext)
	if len(sID) < 4 {
		return ChunkID{}, fmt.Errorf("expected format '/<prefix>/<chunkid>%s", ext)
	}

	// Make sure the prefix does match the first characters of the ID.
	if p != path.Join("/", sID[0:4], sID+ext) {
		return ChunkID{}, fmt.Errorf("expected format '/<prefix>/<chunkid>%s", ext)
	}
	return ChunkIDFromString(sID)
}
