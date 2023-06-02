package roundtrip

import (
	"bytes"
	"context"
	"io"
)

var (
	_ ResponseStreamFilter = (*DefaultResponseFilter)(nil)
	_ ResponseStreamFilter = (*responseBodyWriter)(nil)
)

func NewDefaultResponseFilter() *DefaultResponseFilter {
	return &DefaultResponseFilter{Buffer: bytes.NewBuffer(nil)}
}

// DefaultResponseFilter 的 OnResponseChunk 和 OnResponseEOF 将每一片 chunk 记录在自身的 buffer 中, 没有做其他任何事情.
type DefaultResponseFilter struct {
	*bytes.Buffer
}

// OnResponseChunk 将传入的 chunk 记录在自身的 buffer 中, 同时通过写入 w io.Writer 的方式传递给下一个 ResponseStreamFilter, 没有做其他任何事情.
func (d *DefaultResponseFilter) OnResponseChunk(ctx context.Context, _ HttpInfor, w Writer, chunk []byte) (signal Signal, err error) {
	err = d.multiWrite(w, chunk)
	return map[bool]Signal{true: Continue, false: Intercept}[err == nil], err
}

// OnResponseEOF 将传入的 chunk 记录在自身的 buffer 中, 同时通过写入 w io.Writer 的方式传递给下一个 ResponseStreamFilter, 没有做其他任何事情.
func (d *DefaultResponseFilter) OnResponseEOF(ctx context.Context, _ HttpInfor, w Writer, chunk []byte) (err error) {
	return d.multiWrite(w, chunk)
}

func (d *DefaultResponseFilter) multiWrite(w io.Writer, in []byte) error {
	_, err := io.MultiWriter(d, w).Write(in)
	return err
}

type responseBodyWriter struct {
	written int64
	dst     io.Writer
}

func (r *responseBodyWriter) OnResponseChunk(ctx context.Context, infor HttpInfor, writer Writer, chunk []byte) (signal Signal, err error) {
	return Intercept, r.write(chunk)
}

// OnResponseEOF responseBodyWriter 是一个特殊 ResponseStreamFilter, 它不将数据写入传入的 io.Writer (即传递到下一个 filter),
// 因为它已经没有下一个 filter 了.
// 它直接将数据写入 r.dst, 这个 r.dst 即最终的 response body.
func (r *responseBodyWriter) OnResponseEOF(ctx context.Context, infor HttpInfor, writer Writer, chunk []byte) error {
	return r.write(chunk)
}

func (r *responseBodyWriter) write(in []byte) error {
	n, err := r.dst.Write(in)
	if n > 0 {
		r.written += int64(n)
	}
	if err != nil {
		return err
	}
	if n != len(in) {
		return io.ErrShortWrite
	}
	return nil
}
