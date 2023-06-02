package roundtrip

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

func WrapHTTPHandler(ctx context.Context, handler http.Handler, filters ...RequestFilter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		infor := NewInfor(ctx, r)
		for i := 0; i < len(filters); i++ {
			signal, err := filters[i].OnRequest(ctx, w, infor)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"success": false, "message": %s, "error": %s}`, strconv.Quote(http.StatusText(http.StatusBadRequest)), strconv.Quote(err.Error())), http.StatusBadRequest)
				return
			}
			if signal != Continue {
				return
			}
			r.ContentLength = int64(infor.BodyBuffer().Len())
		}
		handler.ServeHTTP(w, r)
	})
}
