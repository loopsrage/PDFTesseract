package wrapper

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/loopsrage/gotools/lib/httplib"
	"github.com/loopsrage/gotools/lib/index"
	"github.com/loopsrage/gotools/lib/measureTime"
)

const StatusField = "Status"
const ResponseField = "Response"
const SeenField = "Seen"

type ctxKey struct{}

type responseCarrier struct {
	status   int
	response []byte
	set      bool
}

var seen = index.NewIndex()

func Seen(hsh string) bool {
	x, _ := seen.LoadFromIndex(SeenField, hsh)
	switch x.(type) {
	case struct{}:
		return true
	default:
		return false
	}
}

func WriteResponseHeaders(request *http.Request, status *int, response *[]byte) {
	if c, ok := request.Context().Value(ctxKey{}).(*responseCarrier); ok && c != nil {
		if status != nil {
			c.status = *status
		}
		if response != nil {
			c.response = *response
		}
		c.set = true
		return
	}
	if status != nil {
		request.Header.Set(StatusField, strconv.Itoa(*status))
	}
	if response != nil {
		request.Header.Set(ResponseField, string(*response))
	}
}

type HTTPWrapper struct {
	wraps http.HandlerFunc
}

func (h *HTTPWrapper) Serve(w http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	startTime := time.Now()

	defer measureTime.MeasureTime(&startTime)
	defer httplib.WriteCorsResponse(w, response, status)

	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	hsh := fmt.Sprintf("%x", md5.Sum(body))
	if !Seen(hsh) {
		seen.StoreInIndex(SeenField, hsh, struct{}{})
	}

	carrier := &responseCarrier{status: http.StatusOK}
	rWithCarrier := r.WithContext(context.WithValue(r.Context(), ctxKey{}, carrier))

	h.wraps.ServeHTTP(w, rWithCarrier)

	if carrier.set {
		*status = carrier.status
		*response = carrier.response
		return
	}

	if st, err := strconv.ParseInt(rWithCarrier.Header.Get(StatusField), 10, 64); err == nil && st > 0 {
		*status = int(st)
	}
	*response = []byte(rWithCarrier.Header.Get(ResponseField))
}

func NewHTTPWrapper(wraps http.HandlerFunc) *HTTPWrapper {
	return &HTTPWrapper{wraps: wraps}
}
