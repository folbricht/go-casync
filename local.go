package desync

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/folbricht/tempfile"
)

var _ WriteStore = LocalStore{}

const (
	tmpChunkPrefix = ".tmp-cacnk"
)

// LocalStore casync store
type LocalStore struct {
	Base string

	// When accessing chunks, should mtime be updated? Useful when this is
	// a cache. Old chunks can be identified and removed from the store that way
	UpdateTimes bool

	opt StoreOptions

	converters Converters
}

// NewLocalStore creates an instance of a local castore, it only checks presence
// of the store
func NewLocalStore(dir string, opt StoreOptions) (LocalStore, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return LocalStore{}, err
	}
	if !info.IsDir() {
		return LocalStore{}, fmt.Errorf("%s is not a directory", dir)
	}
	converters, err := opt.StorageConverters()
	if err != nil {
		return LocalStore{}, err
	}
	return LocalStore{Base: dir, opt: opt, converters: converters}, nil
}

// GetChunk reads and returns one (compressed!) chunk from the store
func (s LocalStore) GetChunk(id ChunkID) (*Chunk, error) {
	_, p := s.nameFromID(id)
	b, err := ioutil.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, ChunkMissing{id}
	}
	return NewChunkFromStorage(id, b, s.converters, s.opt.SkipVerify)
}

// RemoveChunk deletes a chunk, typically an invalid one, from the filesystem.
// Used when verifying and repairing caches.
func (s LocalStore) RemoveChunk(id ChunkID) error {
	_, p := s.nameFromID(id)
	if _, err := os.Stat(p); err != nil {
		return ChunkMissing{id}
	}
	return os.Remove(p)
}

// StoreChunk adds a new chunk to the store
func (s LocalStore) StoreChunk(chunk *Chunk) error {
	d, p := s.nameFromID(chunk.ID())
	b, err := chunk.Data()
	if err != nil {
		return err
	}
	b, err = s.converters.toStorage(b)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	tmp, err := tempfile.NewMode(d, tmpChunkPrefix, 0644)
	if err != nil {
		return err
	}
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name()) // clean up
		return err
	}
	tmp.Close() // Windows can't rename open files, close explicitly
	return os.Rename(tmp.Name(), p)
}

// Verify all chunks in the store. If repair is set true, bad chunks are deleted.
// n determines the number of concurrent operations. w is used to write any messages
// intended for the user, typically os.Stderr.
func (s LocalStore) Verify(ctx context.Context, n int, repair bool, w io.Writer) error {
	var wg sync.WaitGroup
	ids := make(chan ChunkID)

	// Start the workers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			for id := range ids {
				_, err := s.GetChunk(id)
				switch err.(type) {
				case ChunkInvalid: // bad chunk, report and delete (if repair=true)
					msg := err.Error()
					if repair {
						if err = s.RemoveChunk(id); err != nil {
							msg = msg + ":" + err.Error()
						} else {
							msg = msg + ": removed"
						}
					}
					fmt.Fprintln(w, msg)
				case nil:
				default: // unexpected, print the error and carry on
					fmt.Fprintln(w, err)
				}
			}
			wg.Done()
		}()
	}

	// Go trough all chunks underneath Base, filtering out other files, then feed
	// the IDs to the workers
	extension := s.converters.storageExtension()
	err := filepath.Walk(s.Base, func(path string, info os.FileInfo, err error) error {
		// See if we're meant to stop
		select {
		case <-ctx.Done():
			return Interrupted{}
		default:
		}
		if err != nil { // failed to walk? => fail
			return err
		}
		if info.IsDir() { // Skip dirs
			return nil
		}
		// Skip compressed chunks if this is running in uncompressed mode and vice-versa
		if !strings.HasSuffix(filepath.Base(path), extension) {
			return nil
		}
		sID := strings.TrimSuffix(filepath.Base(path), extension)
		// Convert the name into a checksum, if that fails we're probably not looking
		// at a chunk file and should skip it.
		id, err := ChunkIDFromString(sID)
		if err != nil {
			return nil
		}
		// Feed the workers
		ids <- id
		return nil
	})
	close(ids)
	wg.Wait()
	return err
}

// Prune removes any chunks from the store that are not contained in a list
// of chunks
func (s LocalStore) Prune(ctx context.Context, ids map[ChunkID]struct{}) error {
	extension := s.converters.storageExtension()
	// Go trough all chunks underneath Base, filtering out other directories and files
	err := filepath.Walk(s.Base, func(path string, info os.FileInfo, err error) error {
		// See if we're meant to stop
		select {
		case <-ctx.Done():
			return Interrupted{}
		default:
		}
		if err != nil { // failed to walk? => fail
			return err
		}
		if info.IsDir() { // Skip dirs
			return nil
		}

		// If the chunk is only partially downloaded remove it
		if strings.HasPrefix(filepath.Base(path), tmpChunkPrefix) {
			_ = os.Remove(path)
			return nil
		}
		// Skip compressed chunks if this is running in uncompressed mode and vice-versa
		if !strings.HasSuffix(filepath.Base(path), extension) {
			return nil
		}
		sID := strings.TrimSuffix(filepath.Base(path), extension)
		// Convert the name into a checksum, if that fails we're probably not looking
		// at a chunk file and should skip it.
		id, err := ChunkIDFromString(sID)
		if err != nil {
			return nil
		}
		// See if the chunk we're looking at is in the list we want to keep, if not
		// remove it.
		if _, ok := ids[id]; !ok {
			if err = s.RemoveChunk(id); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// HasChunk returns true if the chunk is in the store
func (s LocalStore) HasChunk(id ChunkID) (bool, error) {
	_, p := s.nameFromID(id)
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s LocalStore) String() string {
	return s.Base
}

// Close the store. NOP opertation, needed to implement Store interface.
func (s LocalStore) Close() error { return nil }

func (s LocalStore) nameFromID(id ChunkID) (dir, name string) {
	sID := id.String()
	dir = filepath.Join(s.Base, sID[0:4])
	name = filepath.Join(dir, sID) + s.converters.storageExtension()
	return
}
