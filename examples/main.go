package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/dspo/roundtrip"
)

func main() {
	mux0 := http.NewServeMux()
	mux0.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("request headers: %+v", r.Header)
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 12; i++ {
			_, _ = w.Write([]byte(fmt.Sprintf("[%d]", i)))
			_, _ = w.Write([]byte(`!{"obnf":!"etqp"-!"bhf":!29}`))
			_, _ = w.Write([]byte{'\n'})
			w.(http.Flusher).Flush()
			time.Sleep(time.Second * 2)
		}
	})
	mux1 := http.NewServeMux()
	mux1.Handle("/", roundtrip.WrapReverseProxy(context.Background(), &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost:8080"
			req.Host = "localhost:8080"
		},
		Transport: &roundtrip.TimerTransport{Inner: &roundtrip.CurlPrinterTransport{}},
	},
		&Filter{Inner: roundtrip.NewDefaultResponseFilter()}))
	go func() {
		if err := http.ListenAndServe(":8080", mux0); err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		if err := http.ListenAndServe(":8081", mux1); err != nil {
			log.Fatalln(err)
		}
	}()
	select {}
}

var _ interface {
	roundtrip.RequestFilter
	roundtrip.ResponseStreamFilter
	roundtrip.ResponseModifierFilter
} = (*Filter)(nil)

type Filter struct {
	Inner *roundtrip.DefaultResponseFilter
}

func (f *Filter) OnRequest(ctx context.Context, w http.ResponseWriter, infor roundtrip.HttpInfor) (signal roundtrip.Signal, err error) {
	if appKey := infor.Header().Get("App-Key"); appKey == "" {
		http.Error(w, "appKey in header is missing", http.StatusUnauthorized)
		return roundtrip.Intercept, nil
	}
	infor.Header().Add("User-Agent", "my-client")
	infor.SetBody(io.NopCloser(bytes.NewBufferString("this is a mocked body")))
	return roundtrip.Continue, nil
}

func (f *Filter) OnResponseChunk(ctx context.Context, infor roundtrip.HttpInfor, w roundtrip.Writer, chunk []byte) (signal roundtrip.Signal, err error) {
	for i := 0; i < len(chunk); i++ {
		if !map[byte]bool{
			'0':  true,
			'1':  true,
			'2':  true,
			'3':  true,
			'4':  true,
			'5':  true,
			'6':  true,
			'7':  true,
			'8':  true,
			'9':  true,
			'{':  true,
			'"':  true,
			':':  true,
			'}':  true,
			'\n': true,
			'[':  true,
			']':  true,
		}[chunk[i]] {
			chunk[i]--
		}
	}
	infor.Header().Add("X-Response-Id", hex.EncodeToString(chunk))
	return f.Inner.OnResponseChunk(ctx, infor, w, chunk)
}

func (f *Filter) OnResponseEOF(ctx context.Context, infor roundtrip.HttpInfor, w roundtrip.Writer, chunk []byte) error {
	infor.Header().Add("X-Response-End-At", time.Now().Format(time.RFC3339))
	return nil
}

func (f *Filter) Modify(ctx context.Context, response *http.Response) error {
	response.Header.Set("X-Reverse-Proxy-End-At", time.Now().Format(time.RFC3339))
	return nil
}
