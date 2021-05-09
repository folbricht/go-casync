package desync

import (
	"net/url"
	"os"
	"path"

	"io"

	"github.com/pkg/errors"
)

// SFTPIndexStore is an index store backed by SFTP over SSH
type SFTPIndexStore struct {
	*SFTPStoreBase
}

// NewSFTPIndexStore initializes and index store backed by SFTP over SSH.
func NewSFTPIndexStore(location *url.URL, opt StoreOptions) (*SFTPIndexStore, error) {
	b, err := newSFTPStoreBase(location, opt, "")
	if err != nil {
		return nil, err
	}
	return &SFTPIndexStore{b}, nil
}

// GetIndexReader returns a reader of an index from an SFTP store. Fails if the specified
// index file does not exist.
func (s *SFTPIndexStore) GetIndexReader(name string) (r io.ReadCloser, e error) {
	f, err := s.client.Open(s.pathFromName(name))
	if err != nil {
		if os.IsNotExist(err) {
			err = errors.Errorf("Index file does not exist: %v", err)
		}
		return r, err
	}
	return f, nil
}

// GetIndex reads an index from an SFTP store, returns an error if the specified index file does not exist.
func (s *SFTPIndexStore) GetIndex(name string) (i Index, e error) {
	f, err := s.GetIndexReader(name)
	if err != nil {
		return i, err
	}
	defer f.Close()
	return IndexFromReader(f)
}

// StoreIndex adds a new index to the store
func (s *SFTPIndexStore) StoreIndex(name string, idx Index) error {
	r, w := io.Pipe()

	go func() {
		defer w.Close()
		idx.WriteTo(w)
	}()
	return s.StoreObject(s.pathFromName(name), r)
}

func (s *SFTPIndexStore) pathFromName(name string) string {
	return path.Join(s.path, name)
}
