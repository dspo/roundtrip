package roundtrip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var (
	_ http.RoundTripper = (*StreamFilterTransport)(nil)
	_ http.RoundTripper = (*DoNothingTransport)(nil)
	_ http.RoundTripper = (*TimerTransport)(nil)
)

type StreamFilterTransport struct {
	Ctx     context.Context
	Filters []ResponseStreamFilter
	Inner   http.Transport
}

func (sft *StreamFilterTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return WrapRoundTrip(sft.Ctx, sft.Inner.RoundTrip, sft.Filters...)(r)
}

type DoNothingTransport struct {
	Response *http.Response
}

func (d *DoNothingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if d.Response == nil {
		d.Response = &http.Response{
			Status:           "",
			StatusCode:       0,
			Proto:            "HTTP/1.1",
			ProtoMajor:       1,
			ProtoMinor:       1,
			Header:           make(http.Header),
			Body:             io.NopCloser(bytes.NewReader(nil)),
			ContentLength:    0,
			TransferEncoding: nil,
			Close:            false,
			Uncompressed:     false,
			Trailer:          nil,
			Request:          req,
			TLS:              nil,
		}
	}
	return d.Response, nil
}

type TimerTransport struct {
	Inner http.RoundTripper
}

func (t *TimerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	begin := time.Now()
	if t.Inner == nil {
		t.Inner = http.DefaultTransport
	}
	res, err := t.Inner.RoundTrip(req)
	log.Printf(Green("[DEBUG]")+" RoundTrip costs %s", time.Now().Sub(begin).String())
	return res, err
}

type CurlPrinterTransport struct {
	Inner http.RoundTripper
}

func (t *CurlPrinterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Inner == nil {
		t.Inner = http.DefaultTransport
	}
	log.Printf(Green("[DEBUG]")+" generated cURL command:\n\t%s\n", GenCurl(req))
	return t.Inner.RoundTrip(req)
}

func GenCurl(req *http.Request) string {
	var curl = fmt.Sprintf(`curl -v -N -X %s '%s://%s%s'`, req.Method, req.URL.Scheme, req.Host, req.URL.RequestURI())
	for k, vv := range req.Header {
		for _, v := range vv {
			curl += fmt.Sprintf(` -H '%s: %s'`, k, v)
		}
	}
	if req.Body != nil {
		var buf = bytes.NewBuffer(nil)
		if _, err := buf.ReadFrom(req.Body); err == nil {
			_ = req.Body.Close()
			curl += ` --data '` + buf.String() + `'`
			req.Body = io.NopCloser(buf)
		}
	}
	return curl
}
