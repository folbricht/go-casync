package desync

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"crypto/x509"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var _ WriteStore = &RemoteHTTP{}

// RemoteHTTPBase is the base object for a remote, HTTP-based chunk or index stores.
type RemoteHTTPBase struct {
	location   *url.URL
	client     *http.Client
	opt        StoreOptions
	converters Converters
}

// RemoteHTTP is a remote casync store accessed via HTTP.
type RemoteHTTP struct {
	*RemoteHTTPBase
}

type GetReaderForRequestBody func() io.Reader

// NewRemoteHTTPStoreBase initializes a base object for HTTP index or chunk stores.
func NewRemoteHTTPStoreBase(location *url.URL, opt StoreOptions) (*RemoteHTTPBase, error) {
	if location.Scheme != "http" && location.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %s, expected http or https", location.Scheme)
	}
	// Make sure we have a trailing / on the path
	if !strings.HasSuffix(location.Path, "/") {
		location.Path = location.Path + "/"
	}

	// Build a TLS client config
	tlsConfig := &tls.Config{InsecureSkipVerify: opt.TrustInsecure}

	// Add client key/cert if provided
	if opt.ClientCert != "" && opt.ClientKey != "" {
		certificate, err := tls.LoadX509KeyPair(opt.ClientCert, opt.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate from %s", opt.ClientCert)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	// Load custom CA set if provided
	if opt.CACert != "" {
		certPool := x509.NewCertPool()
		b, err := ioutil.ReadFile(opt.CACert)
		if err != nil {
			return nil, err
		}
		if ok := certPool.AppendCertsFromPEM(b); !ok {
			return nil, errors.New("no CA certificates found in ca-cert file")
		}
		tlsConfig.RootCAs = certPool
	}

	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DisableCompression:  true,
		MaxIdleConnsPerHost: opt.N,
		IdleConnTimeout:     60 * time.Second,
		TLSClientConfig:     tlsConfig,
		ForceAttemptHTTP2:   true,
	}

	// If no timeout was given in config (set to 0), then use 1 minute. If timeout is negative, use 0 to
	// set an infinite timeout.
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = time.Minute
	} else if timeout < 0 {
		timeout = 0
	}
	client := &http.Client{Transport: tr, Timeout: timeout}

	converters, err := opt.StorageConverters()
	if err != nil {
		return nil, err
	}
	return &RemoteHTTPBase{location: location, client: client, opt: opt, converters: converters}, nil
}

func (r *RemoteHTTPBase) String() string {
	return r.location.String()
}

// Close the HTTP store. NOP operation but needed to implement the interface.
func (r *RemoteHTTPBase) Close() error { return nil }

// Send a single HTTP request.
func (r *RemoteHTTPBase) IssueHttpRequest(method string, u *url.URL, getReader GetReaderForRequestBody, attempt int) (int, []byte, error) {

	var (
		resp *http.Response
		log  = Log.WithFields(logrus.Fields{
			"method":  method,
			"url":     u.String(),
			"attempt": attempt,
		})
	)

	req, err := http.NewRequest(method, u.String(), getReader())
	if err != nil {
		log.Debug("unable to create new request")
		return 0, nil, err
	}
	if r.opt.HTTPAuth != "" {
		req.Header.Set("Authorization", r.opt.HTTPAuth)
	}

	log.Debug("sending request")
	resp, err = r.client.Do(req)
	if err != nil {
		log.WithError(err).Error("error while sending request")
		return 0, nil, errors.Wrap(err, u.String())
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("error while reading response")
		return 0, nil, errors.Wrap(err, u.String())
	}

	log.WithField("statusCode", resp.StatusCode).Debug("response received")
	return resp.StatusCode, b, nil
}

// Send a single HTTP request, retrying if a retryable error has occurred.
func (r *RemoteHTTPBase) IssueRetryableHttpRequest(method string, u *url.URL, getReader GetReaderForRequestBody) (int, []byte, error) {

	var (
		attempt int
		log     = Log.WithFields(logrus.Fields{
			"method": method,
			"url":    u.String(),
		})
	)

retry:
	attempt++
	statusCode, responseBody, err := r.IssueHttpRequest(method, u, getReader, attempt)
	if (err != nil) || (statusCode >= 500 && statusCode < 600) {
		if attempt >= r.opt.ErrorRetry {
			log.WithField("attempt", attempt).Debug("failed, giving up")
			return statusCode, responseBody, err
		} else {
			log.WithField("attempt", attempt).WithField("delay", attempt).Debug("waiting, then retrying")
			time.Sleep(time.Duration(attempt) * r.opt.ErrorRetryBaseInterval)
			goto retry
		}
	}

	return statusCode, responseBody, nil
}

// GetObject reads and returns an object in the form of []byte from the store
func (r *RemoteHTTPBase) GetObject(name string) ([]byte, error) {
	u, _ := r.location.Parse(name)
	statusCode, responseBody, err := r.IssueRetryableHttpRequest("GET", u, func() io.Reader { return nil })
	if err != nil {
		return nil, err
	}
	switch statusCode {
	case 200: // expected
		return responseBody, nil
	case 404:
		return nil, NoSuchObject{name}
	default:
		return nil, fmt.Errorf("unexpected status code %d from %s", statusCode, name)
	}
}

// StoreObject stores an object to the store.
func (r *RemoteHTTPBase) StoreObject(name string, getReader GetReaderForRequestBody) error {
	u, _ := r.location.Parse(name)
	statusCode, responseBody, err := r.IssueRetryableHttpRequest("PUT", u, getReader)
	if err != nil {
		return err
	}
	if statusCode != 200 {
		return errors.New(string(responseBody))
	}
	return nil
}

// NewRemoteHTTPStore initializes a new store that pulls chunks via HTTP(S) from
// a remote web server. n defines the size of idle connections allowed.
func NewRemoteHTTPStore(location *url.URL, opt StoreOptions) (*RemoteHTTP, error) {
	b, err := NewRemoteHTTPStoreBase(location, opt)
	if err != nil {
		return nil, err
	}
	return &RemoteHTTP{b}, nil
}

// GetChunk reads and returns one chunk from the store
func (r *RemoteHTTP) GetChunk(id ChunkID) (*Chunk, error) {
	p := r.nameFromID(id)
	b, err := r.GetObject(p)
	if err != nil {
		return nil, err
	}
	return NewChunkFromStorage(id, b, r.converters, r.opt.SkipVerify)
}

// HasChunk returns true if the chunk is in the store
func (r *RemoteHTTP) HasChunk(id ChunkID) (bool, error) {
	p := r.nameFromID(id)
	u, _ := r.location.Parse(p)

	statusCode, _, err := r.IssueRetryableHttpRequest("HEAD", u, func() io.Reader { return nil })
	if err != nil {
		return false, err
	}
	switch statusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code: %d", statusCode)
	}
}

// StoreChunk adds a new chunk to the store
func (r *RemoteHTTP) StoreChunk(chunk *Chunk) error {
	p := r.nameFromID(chunk.ID())
	b, err := chunk.Data()
	if err != nil {
		return err
	}
	b, err = r.converters.toStorage(b)
	if err != nil {
		return err
	}
	return r.StoreObject(p, func() io.Reader { return bytes.NewReader(b) })
}

func (r *RemoteHTTP) nameFromID(id ChunkID) string {
	sID := id.String()
	name := path.Join(sID[0:4], sID) + r.converters.storageExtension()
	return name
}
