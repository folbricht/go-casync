// +build !datadog

package desync

import "github.com/klauspost/compress/zstd"

// Create a reader/writer that caches compressors.
var (
	encoder, _ = zstd.NewWriter(nil)
	decoder, _ = zstd.NewReader(nil)
)

// Compress a block using the only (currently) supported algorithm
func Compress(src []byte) ([]byte, error) {
	return encoder.EncodeAll(src, make([]byte, 0, len(src))), nil
}

// Decompress a block using the only supported algorithm. If you already have
// a buffer it can be passed into out and will be used. If out=nil, a buffer
// will be allocated.
func Decompress(dst, src []byte) ([]byte, error) {
	return decoder.DecodeAll(src, dst)
}

// Compression layer converter. Compresses/decompresses chunk data
// to and from storage. Implements the converter interface.
type Compressor struct{}

var _ converter = Compressor{}

func (d Compressor) toStorage(in []byte) ([]byte, error) {
	return Compress(in)
}

func (d Compressor) fromStorage(in []byte) ([]byte, error) {
	return Decompress(nil, in)
}

func (d Compressor) equal(c converter) bool {
	_, ok := c.(Compressor)
	return ok
}

func (d Compressor) storageExtension() string {
	return ".cacnk"
}
