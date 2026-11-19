package pdf

import (
	"encoding/json"
	"errors"
	"net/http"

	"Tesseract/handlers"
	"Tesseract/wrapper"

	"github.com/loopsrage/gotools/lib/docpipeline"
	"github.com/loopsrage/gotools/lib/visionlib"
)

// PDF serves /PDF requests by routing PDF uploads into the unified indexQueue pipeline.
type PDF struct {
	engine *docpipeline.Engine
}

// NewPDF constructs a PDF endpoint handler backed by engine.
func NewPDF(engine *docpipeline.Engine) *PDF {
	return &PDF{engine: engine}
}

// ServePDF parses a PDF upload, executes the indexQueue pipeline, and returns the persisted document summary.
func (p *PDF) ServePDF(_ http.ResponseWriter, r *http.Request) {
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

	kind, _, err := visionlib.DetectInputKind(req.Data, req.MimeType)
	if err != nil {
		*status = http.StatusBadRequest
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		*response = b
		return
	}
	if kind != visionlib.InputPDF {
		*status = http.StatusBadRequest
		b, _ := json.Marshal(map[string]string{"error": errors.New("expected PDF payload on /PDF").Error()})
		*response = b
		return
	}

	res, err := p.engine.Process(r.Context(), req)
	if err != nil {
		*status = http.StatusBadRequest
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		*response = b
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
