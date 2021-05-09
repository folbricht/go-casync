package main

import (
	"context"

	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/folbricht/desync"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type chunkServerOptions struct {
	cmdStoreOptions
	cmdServerOptions
	stores          []string
	cache           string
	storeFile       string
	listenAddresses []string
	writable        bool
	skipVerifyWrite bool
	uncompressed    bool
	logFile         string
	encryptionAlg   string
	encryptionPw    string
}

func newChunkServerCommand(ctx context.Context) *cobra.Command {
	var opt chunkServerOptions

	cmd := &cobra.Command{
		Use:   "chunk-server",
		Short: "Server for chunks over HTTP(S)",
		Long: `Starts an HTTP chunk server that can be used as remote store. It supports
reading from multiple local or remote stores as well as a local cache. If
--cert and --key are provided, the server will serve over HTTPS. The -w option
enables writing to this store, but this is only allowed when just one upstream
chunk store is provided. The option --skip-verify-write disables validation of
chunks written to this server which bypasses checksum validation as well as
the necessary decompression step to calculate it to improve performance. If -u
is used, only uncompressed chunks are being served (and accepted). If the
upstream store serves compressed chunks, everything will have to be decompressed 
server-side so it's better to also read from uncompressed upstream stores.

While --concurrency does not limit the number of clients that can be served
concurrently, it does influence connection pools to remote upstream stores and
needs to be chosen carefully if the server is under high load.

This command supports the --store-file option which can be used to define the stores
and caches in a JSON file. The config can then be reloaded by sending a SIGHUP without
needing to restart the server. This can be done under load as well.
`,
		Example: `  desync chunk-server -s sftp://192.168.1.1/store -c /path/to/cache -l :8080`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChunkServer(ctx, opt, args)
		},
		SilenceUsage: true,
	}
	flags := cmd.Flags()
	flags.StringVar(&opt.storeFile, "store-file", "", "read store arguments from a file, supports reload on SIGHUP")
	flags.StringSliceVarP(&opt.stores, "store", "s", nil, "upstream source store(s)")
	flags.StringVarP(&opt.cache, "cache", "c", "", "store to be used as cache")
	flags.StringSliceVarP(&opt.listenAddresses, "listen", "l", []string{":http"}, "listen address")
	flags.BoolVarP(&opt.writable, "writeable", "w", false, "support writing")
	flags.BoolVar(&opt.skipVerify, "skip-verify-read", true, "don't verify chunk data read from upstream stores (faster)")
	flags.BoolVar(&opt.skipVerifyWrite, "skip-verify-write", true, "don't verify chunk data written to this server (faster)")
	flags.BoolVarP(&opt.uncompressed, "uncompressed", "u", false, "serve uncompressed chunks")
	flags.StringVar(&opt.logFile, "log", "", "request log file or - for STDOUT")
	flags.StringVar(&opt.encryptionPw, "encryption-password", "", "serve chunks encrypted with this password")
	flags.StringVar(&opt.encryptionAlg, "encryption-algorithm", "xchacha20-poly1305", "encryption algorithm")
	addStoreOptions(&opt.cmdStoreOptions, flags)
	addServerOptions(&opt.cmdServerOptions, flags)
	return cmd
}

func runChunkServer(ctx context.Context, opt chunkServerOptions, args []string) error {
	if err := opt.cmdStoreOptions.validate(); err != nil {
		return err
	}
	if err := opt.cmdServerOptions.validate(); err != nil {
		return err
	}
	if opt.auth == "" {
		opt.auth = os.Getenv("DESYNC_HTTP_AUTH")
	}

	addresses := opt.listenAddresses
	if len(addresses) == 0 {
		addresses = []string{":http"}
	}

	// Extract the store setup from command line options and validate it
	s, err := chunkServerStore(opt)
	if err != nil {
		return err
	}

	// When a store file is used, it's possible to reload the store setup from it
	// on the fly. Wrap the store into a SwapStore and start a handler for SIGHUP,
	// reloading the store config from file.
	if opt.storeFile != "" {
		if _, ok := s.(desync.WriteStore); ok {
			s = desync.NewSwapWriteStore(s)
		} else {
			s = desync.NewSwapStore(s)
		}

		go func() {
			for range sighup {
				newStore, err := chunkServerStore(opt)
				if err != nil {
					fmt.Fprintln(stderr, "failed to reload configuration:", err)
					continue
				}
				switch store := s.(type) {
				case *desync.SwapStore:
					if err := store.Swap(newStore); err != nil {
						fmt.Fprintln(stderr, "failed to reload configuration:", err)
					}
				case *desync.SwapWriteStore:
					if err := store.Swap(newStore); err != nil {
						fmt.Fprintln(stderr, "failed to reload configuration:", err)
					}
				}
			}
		}()
	}
	defer s.Close()

	// Build the converters. In this case, the "storage" side is what is served
	// up by the server towards the client. The StoreOptions struct already has
	// logic to build the converters from options so use that instead of repeating
	// it here.
	converters, err := desync.StoreOptions{
		Uncompressed:        opt.uncompressed,
		Encryption:          opt.encryptionPw != "",
		EncryptionAlgorithm: opt.encryptionAlg,
		EncryptionPassword:  opt.encryptionPw,
	}.StorageConverters()
	if err != nil {
		return err
	}

	handler := desync.NewHTTPHandler(s, opt.writable, opt.skipVerifyWrite, converters, opt.auth)

	// Wrap the handler in a logger if requested
	switch opt.logFile {
	case "": // No logging of requests
	case "-":
		handler = withLog(handler, log.New(stderr, "", log.LstdFlags))
	default:
		l, err := os.OpenFile(opt.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer l.Close()
		handler = withLog(handler, log.New(l, "", log.LstdFlags))
	}

	http.Handle("/", handler)

	// Start the server
	return serve(ctx, opt.cmdServerOptions, addresses...)
}

// Wrapper for http.HandlerFunc to add logging for requests (and response codes)
func withLog(h http.Handler, log *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w}
		h.ServeHTTP(lrw, r)
		log.Printf("Client: %s, Request: %s %s, Response: %d", r.RemoteAddr, r.Method, r.RequestURI, lrw.statusCode)
	}
}

// Reads the store-related command line options and returns the appropriate store.
func chunkServerStore(opt chunkServerOptions) (desync.Store, error) {
	stores := opt.stores
	cache := opt.cache

	var err error
	if opt.storeFile != "" {
		if len(stores) != 0 {
			return nil, errors.New("--store and --store-file can't be used together")
		}
		if cache != "" {
			return nil, errors.New("--cache and --store-file can't be used together")
		}
		stores, cache, err = readStoreFile(opt.storeFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read store-file '%s'", err)
		}
	}

	// Got to have at least one upstream store
	if len(stores) == 0 {
		return nil, errors.New("no store provided")
	}

	// When supporting writing, only one upstream store is possible and no cache
	if opt.writable && (len(stores) > 1 || cache != "") {
		return nil, errors.New("Only one upstream store supported for writing and no cache")
	}

	var s desync.Store
	if opt.writable {
		s, err = WritableStore(stores[0], opt.cmdStoreOptions)
		if err != nil {
			return nil, err
		}
	} else {
		s, err = MultiStoreWithCache(opt.cmdStoreOptions, cache, stores...)
		if err != nil {
			return nil, err
		}
		// We want to take the edge of a large number of requests coming in for the same chunk. No need
		// to hit the (potentially slow) upstream stores for duplicated requests.
		s = desync.NewDedupQueue(s)
	}
	return s, nil
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
