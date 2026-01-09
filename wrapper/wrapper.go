package wrapper

import (
	"Tesseract/lib/httplib"
	"Tesseract/lib/index"
	"Tesseract/lib/measureTime"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const StatusField = "Status"
const ResponseField = "Response"
const SeenField = "Seen"

var mux = http.NewServeMux()
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
	request.Header.Set(StatusField, strconv.Itoa(*status))
	request.Header.Set(ResponseField, string(*response))
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
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	hsh := fmt.Sprintf("%x", md5.Sum(body))

	if !Seen(hsh) {
		seen.StoreInIndex("seen", hsh, struct{}{})
	}

	h.wraps.ServeHTTP(w, r)

	st, _ := strconv.ParseInt(r.Header.Get(StatusField), 10, 64)
	*status = int(st)
	*response = []byte(r.Header.Get(ResponseField))
}

func NewHTTPWrapper(wraps http.HandlerFunc) *HTTPWrapper {
	return &HTTPWrapper{
		wraps: wraps,
	}
}
