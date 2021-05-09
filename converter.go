package desync

import "strings"

// Converters are modifiers for chunk data, such as compression or encryption.
// They are used to prepare chunk data for storage, or to read it from storage.
// The order of the conversion layers matters. When plain data is prepared for
// storage, the toStorage method is used in the order the layers are defined.
// To read from storage, the fromStorage method is called for each layer in
// reverse order.
type Converters []converter

// Apply every data converter in the forward direction.
func (s Converters) toStorage(in []byte) ([]byte, error) {
	var (
		b   = in
		err error
	)
	for _, layer := range s {
		b, err = layer.toStorage(b)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Apply the layers backwards.
func (s Converters) fromStorage(in []byte) ([]byte, error) {
	var (
		b   = in
		err error
	)
	for i := len(s) - 1; i >= 0; i-- {
		b, err = s[i].fromStorage(b)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Returns true is conversion involves compression. Typically
// used to determine the correct file-extension.
func (s Converters) hasCompression() bool {
	for _, layer := range s {
		if _, ok := layer.(Compressor); ok {
			return true
		}
	}
	return false
}

// Returns true if both converters have the same layers in the
// same order. Used for optimizations.
func (s Converters) equal(c Converters) bool {
	if len(s) != len(c) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !s[i].equal(c[i]) {
			return false
		}
	}
	return true
}

// Extension to be used in storage. Concatenation of converter
// extensions in order (towards storage).
func (s Converters) storageExtension() string {
	var ext strings.Builder
	for _, layer := range s {
		ext.WriteString(layer.storageExtension())
	}
	return ext.String()
}

// converter is a storage data modifier layer.
type converter interface {
	// Convert data from it's original form to storage format.
	// The input could be plain data, or the output of a prior
	// converter.
	toStorage([]byte) ([]byte, error)

	// Convert data from it's storage format towards it's plain
	// form. The input could be encrypted or compressed, while
	// the output may be used for the next conversion layer.
	fromStorage([]byte) ([]byte, error)

	// Returns the file extension that should be used for a
	// chunk when stored. Usually a concatenation of layers.
	storageExtension() string

	// True is one converter matches another exactly.
	equal(converter) bool
}
