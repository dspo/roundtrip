package roundtrip

import (
	"context"
	"net/http"
	"net/http/httputil"
)

func WrapReverseProxy(ctx context.Context, proxy *httputil.ReverseProxy, filters ...Filter) http.Handler {
	if proxy.Director == nil {
		proxy.Director = DoNothingDirector
		proxy.Transport = &DoNothingTransport{}
		return proxy
	}
	if proxy.BufferPool == nil {
		proxy.BufferPool = DefaultBufferPool
	}
	var (
		requestFilters  []RequestFilter
		streamFilters   []ResponseStreamFilter
		modifierFilters []ResponseModifierFilter
	)
	for i := 0; i < len(filters); i++ {
		if filter, ok := filters[i].(RequestFilter); ok {
			requestFilters = append(requestFilters, filter)
		}
		if filter, ok := filters[len(filters)-1-i].(ResponseStreamFilter); ok {
			streamFilters = append(streamFilters, filter)
		}
		if filter, ok := filters[len(filters)-1-i].(ResponseModifierFilter); ok {
			modifierFilters = append(modifierFilters, filter)
		}
	}
	proxy.Transport = WrapRoundTripper(ctx, proxy.Transport, streamFilters...)
	proxy.ModifyResponse = WrapModifier(ctx, proxy.ModifyResponse, modifierFilters...)
	return WrapHTTPHandler(ctx, proxy, requestFilters...)
}
