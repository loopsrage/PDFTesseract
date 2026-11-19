package tesseract

import (
	"encoding/json"
	"net/http"
	"strings"

	"Tesseract/handlers"
	"Tesseract/wrapper"

	"github.com/loopsrage/gotools/lib/docpipeline"
)

// Tesseract serves /Tesseract requests by routing images (or PDFs) into the unified indexQueue pipeline
// with swappable OCR providers and proceduralphaseserver persistence.
type Tesseract struct {
	engine *docpipeline.Engine
}

// NewTesseract constructs a Tesseract endpoint handler backed by engine.
func NewTesseract(engine *docpipeline.Engine) *Tesseract {
	return &Tesseract{engine: engine}
}

// ServeTesseract executes the unified segment -> denoise -> OCR -> proceduralphaseserver pipeline.
func (t *Tesseract) ServeTesseract(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	req, err := handlers.ParseSubmitRequest(r)
	if err != nil {
		*status = http.StatusBadRequest
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		*response = b
		return
	}

	res, err := t.engine.Process(r.Context(), req)
	if err != nil {
		*status = http.StatusBadRequest
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		*response = b
		return
	}

	if strings.EqualFold(r.URL.Query().Get("format"), "text") {
		*response = []byte(res.FullText)
		return
	}
	b, err := json.Marshal(res)
	if err != nil {
		*status = http.StatusInternalServerError
		*response = []byte(err.Error())
		return
	}
	*response = b
}
