package roundtrip

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func WrapRoundTripper(ctx context.Context, tripper http.RoundTripper, filters ...ResponseStreamFilter) http.RoundTripper {
	if tripper == nil {
		tripper = http.DefaultTransport
	}
	return &roundTripper{roundTrip: func() func(r *http.Request) (*http.Response, error) {
		return WrapRoundTrip(ctx, tripper.RoundTrip, filters...)
	}}
}

func WrapRoundTrip(ctx context.Context, roundTrip func(req *http.Request) (*http.Response, error), filters ...ResponseStreamFilter) func(r *http.Request) (*http.Response, error) {
	if roundTrip == nil {
		roundTrip = http.DefaultTransport.RoundTrip
	}
	return func(req *http.Request) (*http.Response, error) {
		resp, err := roundTrip(req)
		if err != nil {
			return resp, err
		}
		var (
			src    io.ReadCloser
			pr, pw = io.Pipe()
			buf    = DefaultBufferPool.Get()
		)
		src, resp.Body = resp.Body, pr
		filters = append(filters, &responseBodyWriter{dst: pw})
		go func() {
			defer func() {
				_ = src.Close()
				_ = pw.Close()
				DefaultBufferPool.Put(buf)
			}()
			var nextWriter bytes.Buffer
			for {
				nr, rerr := src.Read(buf)
				if rerr != nil && rerr != io.EOF && rerr != context.Canceled {
					log.Printf(Yellow("WARN")+"RoundTrip read error during body copy: %v", rerr)
				}
				if nr > 0 {
					nextReader := buf[:nr]
					for i := 0; i < len(filters); i++ {
						_, err := filters[i].OnResponseChunk(ctx, NewInfor(ctx, resp), &nextWriter, nextReader)
						if err != nil {
							log.Printf(Red("[ERROR]")+"failed to %T.OnResponseChunk, err: %v", filters[0], err)
							return
						}
						nextReader = nextWriter.Bytes()
						nextWriter.Reset()
					}
				}
				if rerr != nil {
					if rerr == io.EOF && (strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") || strings.EqualFold(os.Getenv("DEBUG"), "true")) {
						log.Printf(Green("[DEBUG]")+"failed to src.Read, err: %v", rerr)
					} else {
						log.Printf(Red("[ERROR]")+"failed to src.Read, err: %v", rerr)
					}
					var nextReader []byte
					nextWriter.Reset()
					for i := 0; i < len(filters); i++ {
						if err := filters[i].OnResponseEOF(ctx, NewInfor(ctx, resp), &nextWriter, nextReader); err != nil {
							log.Printf(Red("[ERROR]")+"failed to %T.OnResponseEOF, err: %v", filters[0], err)
							return
						}
						nextReader = nextWriter.Bytes()
						nextWriter.Reset()
					}
					return
				}
			}
		}()
		return resp, nil
	}
}

func WrapModifier(ctx context.Context, modify func(resp *http.Response) error, filters ...ResponseModifierFilter) func(response *http.Response) error {
	return func(resp *http.Response) error {
		if modify != nil {
			if err := modify(resp); err != nil {
				return err
			}
		}
		for _, filter := range filters {
			if err := filter.Modify(ctx, resp); err != nil {
				return err
			}
		}
		return nil
	}
}

type roundTripper struct {
	roundTrip func() func(r *http.Request) (*http.Response, error)
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.roundTrip()(req)
}
