package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"Tesseract/config"
	"Tesseract/wrapper"

	"github.com/loopsrage/gotools/lib/docpipeline"
	"github.com/loopsrage/gotools/lib/ocrlib"
	"github.com/loopsrage/gotools/lib/visionlib"
)

// API bundles unified HTTP handlers backed by docpipeline.Engine and proceduralphaseserver.
type API struct {
	engine *docpipeline.Engine
}

// NewAPI constructs an HTTP API bound to engine.
func NewAPI(engine *docpipeline.Engine) *API {
	return &API{engine: engine}
}

// RegisterRoutes mounts all unified pipeline, document hierarchy, provider swap, and health endpoints on mux.
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/Process", wrapper.NewHTTPWrapper(a.ServeProcess).Serve)
	mux.HandleFunc("/Image", wrapper.NewHTTPWrapper(a.ServeImage).Serve)
	mux.HandleFunc("/Documents/", wrapper.NewHTTPWrapper(a.ServeDocumentHierarchy).Serve)
	mux.HandleFunc("/Providers", wrapper.NewHTTPWrapper(a.ServeProviders).Serve)
	mux.HandleFunc("/Providers/Default", wrapper.NewHTTPWrapper(a.ServeSetDefaultProvider).Serve)
	mux.HandleFunc("/Providers/HTTP", wrapper.NewHTTPWrapper(a.ServeRegisterHTTPProvider).Serve)
	mux.HandleFunc("/Health", wrapper.NewHTTPWrapper(a.ServeHealth).Serve)
}

// ServeProcess accepts either a PDF or an Image (JSON, multipart, or raw bytes) and runs the indexQueue pipeline.
func (a *API) ServeProcess(w http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	if r.Method != http.MethodPost {
		writeErr(status, response, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	req, err := ParseSubmitRequest(r)
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}

	res, err := a.engine.Process(r.Context(), req)
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}

	if strings.EqualFold(r.URL.Query().Get("format"), "text") {
		*response = []byte(res.FullText)
		return
	}
	b, err := json.Marshal(res)
	if err != nil {
		writeErr(status, response, http.StatusInternalServerError, err)
		return
	}
	*response = b
}

// ServeImage validates that the uploaded payload is an image and runs it through the unified post-render pipeline.
func (a *API) ServeImage(w http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	if r.Method != http.MethodPost {
		writeErr(status, response, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	req, err := ParseSubmitRequest(r)
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}

	kind, _, err := visionlib.DetectInputKind(req.Data, req.MimeType)
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	if kind != visionlib.InputImage {
		writeErr(status, response, http.StatusBadRequest, errors.New("expected image file on /Image (use /Process or /PDF for PDF files)"))
		return
	}

	res, err := a.engine.Process(r.Context(), req)
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	b, err := json.Marshal(res)
	if err != nil {
		writeErr(status, response, http.StatusInternalServerError, err)
		return
	}
	*response = b
}

// ServeDocumentHierarchy retrieves the hierarchical JSON document tree for /Documents/{doc_id} from proceduralphaseserver.
func (a *API) ServeDocumentHierarchy(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	docID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/Documents/"), "/")
	if docID == "" {
		writeErr(status, response, http.StatusBadRequest, errors.New("missing doc_id in /Documents/{doc_id}"))
		return
	}
	includeBlobs := parseBoolQuery(r.URL.Query().Get("include_blobs"), false)

	view, err := a.engine.PPSClient().GetDocumentHierarchy(r.Context(), docID, includeBlobs)
	if err != nil {
		writeErr(status, response, http.StatusBadGateway, err)
		return
	}
	if view.RecordCount == 0 {
		writeErr(status, response, http.StatusNotFound, errors.New("document not found in proceduralphaseserver"))
		return
	}
	b, _ := json.Marshal(view)
	*response = b
}

// ServeProviders lists all registered swappable OCR models and the current default.
func (a *API) ServeProviders(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	b, _ := json.Marshal(map[string]any{
		"default":   a.engine.OCRRegistry().DefaultName(),
		"providers": a.engine.OCRRegistry().List(),
	})
	*response = b
}

// ServeSetDefaultProvider hot-swaps the active default OCR provider.
func (a *API) ServeSetDefaultProvider(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	if r.Method != http.MethodPost {
		writeErr(status, response, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var body struct {
		Provider string `json:"provider"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	target := strings.TrimSpace(body.Provider)
	if target == "" {
		target = strings.TrimSpace(r.URL.Query().Get("provider"))
	}
	if err := a.engine.OCRRegistry().SetDefault(target); err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	b, _ := json.Marshal(map[string]any{
		"default":   a.engine.OCRRegistry().DefaultName(),
		"providers": a.engine.OCRRegistry().List(),
	})
	*response = b
}

// ServeRegisterHTTPProvider dynamically registers (and optionally activates) a remote HTTP OCR model endpoint.
func (a *API) ServeRegisterHTTPProvider(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	if r.Method != http.MethodPost {
		writeErr(status, response, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var body struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		ModelID    string `json:"model_id"`
		APIKey     string `json:"api_key"`
		TimeoutMs  int    `json:"timeout_ms"`
		SetDefault bool   `json:"set_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	hp, err := ocrlib.NewHTTPProvider(config.HTTPProviderConfig{
		Enabled:   true,
		Name:      body.Name,
		Endpoint:  body.Endpoint,
		ModelID:   body.ModelID,
		APIKey:    body.APIKey,
		TimeoutMs: body.TimeoutMs,
	})
	if err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	if err := a.engine.OCRRegistry().Register(hp); err != nil {
		writeErr(status, response, http.StatusBadRequest, err)
		return
	}
	if body.SetDefault {
		_ = a.engine.OCRRegistry().SetDefault(hp.Name())
	}
	b, _ := json.Marshal(map[string]any{
		"registered": hp.Name(),
		"default":    a.engine.OCRRegistry().DefaultName(),
		"providers":  a.engine.OCRRegistry().List(),
	})
	*response = b
}

// ServeHealth returns indexQueue telemetry, Abacus counters, OCR providers, and proceduralphaseserver status.
func (a *API) ServeHealth(_ http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	ppsStatus, ppsErr := a.engine.PPSClient().Status(r.Context())
	out := map[string]any{
		"status":   "ok",
		"pipeline": a.engine.Stats(),
		"pps":      ppsStatus,
	}
	if ppsErr != nil {
		out["pps_error"] = ppsErr.Error()
	}
	b, _ := json.Marshal(out)
	*response = b
}

type jsonSubmitBody struct {
	DocID         string   `json:"DocID"`
	DocIDAlt      string   `json:"doc_id"`
	Data          string   `json:"Data"`
	DataAlt       string   `json:"data"`
	Name          string   `json:"Name"`
	NameAlt       string   `json:"file_name"`
	MimeType      string   `json:"MimeType"`
	MimeTypeAlt   string   `json:"mime_type"`
	OCRProvider   string   `json:"OCRProvider"`
	OCRProvAlt    string   `json:"ocr_provider"`
	OCRProviders  []string `json:"OCRProviders"`
	OCRProvsAlt   []string `json:"ocr_providers"`
	DenoisePreset string   `json:"DenoisePreset"`
	DenoiseAlt    string   `json:"denoise_preset"`
	Segment       *bool    `json:"Segment"`
	SegmentAlt    *bool    `json:"segment"`
	DPI           int      `json:"DPI"`
	DPIAlt        int      `json:"dpi"`
	Language      string   `json:"Language"`
	LanguageAlt   string   `json:"language"`
	FlushPT       *bool    `json:"FlushPT"`
	FlushPTAlt    *bool    `json:"flush_pt"`
}

// ParseSubmitRequest decodes a multipart/form-data, JSON DataWrapper, or raw binary HTTP upload into SubmitRequest.
func ParseSubmitRequest(r *http.Request) (docpipeline.SubmitRequest, error) {
	var out docpipeline.SubmitRequest
	q := r.URL.Query()

	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return out, err
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			return out, err
		}
		defer file.Close()
		raw, err := io.ReadAll(file)
		if err != nil {
			return out, err
		}
		out.Data = raw
		out.FileName = hdr.Filename
		out.MimeType = hdr.Header.Get("Content-Type")
		out.DocID = firstNonEmpty(r.FormValue("doc_id"), q.Get("doc_id"))
		out.DenoisePreset = visionlib.DenoisePreset(firstNonEmpty(r.FormValue("denoise_preset"), q.Get("denoise_preset")))
		out.Language = firstNonEmpty(r.FormValue("language"), q.Get("language"))
		if dpi, err := strconv.Atoi(firstNonEmpty(r.FormValue("dpi"), q.Get("dpi"))); err == nil && dpi > 0 {
			out.DPI = dpi
		}
		if segStr := firstNonEmpty(r.FormValue("segment"), q.Get("segment")); segStr != "" {
			v := parseBoolQuery(segStr, true)
			out.Segment = &v
		}
		out.FlushPT = parseBoolQuery(firstNonEmpty(r.FormValue("flush_pt"), q.Get("flush_pt")), false)
		out.OCRProviders = splitProviders(firstNonEmpty(r.FormValue("ocr_providers"), r.FormValue("ocr_provider"), q.Get("ocr_providers"), q.Get("ocr_provider")))
		return out, nil
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return out, err
	}
	trimmed := bytes.TrimSpace(rawBody)
	if len(trimmed) == 0 {
		return out, errors.New("empty request body")
	}

	// Detect JSON object upload ({ "Data": "...", "Name": "..." })
	if trimmed[0] == '{' {
		var jb jsonSubmitBody
		if err := json.Unmarshal(trimmed, &jb); err == nil && (jb.Data != "" || jb.DataAlt != "") {
			b64Str := firstNonEmpty(jb.Data, jb.DataAlt)
			decoded, err := decodeFlexibleBase64(b64Str)
			if err != nil {
				return out, err
			}
			out.Data = decoded
			out.DocID = firstNonEmpty(jb.DocID, jb.DocIDAlt, q.Get("doc_id"))
			out.FileName = firstNonEmpty(jb.Name, jb.NameAlt, q.Get("name"))
			out.MimeType = firstNonEmpty(jb.MimeType, jb.MimeTypeAlt)
			out.DenoisePreset = visionlib.DenoisePreset(firstNonEmpty(jb.DenoisePreset, jb.DenoiseAlt, q.Get("denoise_preset")))
			out.Language = firstNonEmpty(jb.Language, jb.LanguageAlt, q.Get("language"))
			if jb.DPI > 0 {
				out.DPI = jb.DPI
			} else if jb.DPIAlt > 0 {
				out.DPI = jb.DPIAlt
			} else if dpi, err := strconv.Atoi(q.Get("dpi")); err == nil && dpi > 0 {
				out.DPI = dpi
			}
			if jb.Segment != nil {
				out.Segment = jb.Segment
			} else if jb.SegmentAlt != nil {
				out.Segment = jb.SegmentAlt
			} else if segStr := q.Get("segment"); segStr != "" {
				v := parseBoolQuery(segStr, true)
				out.Segment = &v
			}
			if jb.FlushPT != nil {
				out.FlushPT = *jb.FlushPT
			} else if jb.FlushPTAlt != nil {
				out.FlushPT = *jb.FlushPTAlt
			} else {
				out.FlushPT = parseBoolQuery(q.Get("flush_pt"), false)
			}
			provs := append([]string{}, jb.OCRProviders...)
			provs = append(provs, jb.OCRProvsAlt...)
			provs = append(provs, splitProviders(firstNonEmpty(jb.OCRProvider, jb.OCRProvAlt, q.Get("ocr_providers"), q.Get("ocr_provider")))...)
			out.OCRProviders = provs
			return out, nil
		}
	}

	// Raw binary PDF / Image body
	out.Data = rawBody
	out.DocID = q.Get("doc_id")
	out.FileName = q.Get("name")
	out.MimeType = r.Header.Get("Content-Type")
	out.DenoisePreset = visionlib.DenoisePreset(q.Get("denoise_preset"))
	out.Language = q.Get("language")
	if dpi, err := strconv.Atoi(q.Get("dpi")); err == nil && dpi > 0 {
		out.DPI = dpi
	}
	if segStr := q.Get("segment"); segStr != "" {
		v := parseBoolQuery(segStr, true)
		out.Segment = &v
	}
	out.FlushPT = parseBoolQuery(q.Get("flush_pt"), false)
	out.OCRProviders = splitProviders(firstNonEmpty(q.Get("ocr_providers"), q.Get("ocr_provider"), r.Header.Get("X-OCR-Provider")))
	return out, nil
}

// decodeFlexibleBase64 handles standard Base64, URL Base64, data-URI prefixes, and legacy double-Base64 payloads.
func decodeFlexibleBase64(s string) ([]byte, error) {
	clean := strings.TrimSpace(s)
	if idx := strings.Index(clean, ";base64,"); idx >= 0 {
		clean = clean[idx+len(";base64,"):]
	}
	dec, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		dec, err = base64.RawStdEncoding.DecodeString(clean)
		if err != nil {
			return nil, err
		}
	}
	// Handle legacy double-encoded base64 (where JSON []byte marshaled an already-base64 string)
	if _, _, kindErr := visionlib.DetectInputKind(dec, ""); kindErr != nil {
		if inner, err2 := base64.StdEncoding.DecodeString(strings.TrimSpace(string(dec))); err2 == nil {
			if _, _, kindErr2 := visionlib.DetectInputKind(inner, ""); kindErr2 == nil {
				return inner, nil
			}
		}
	}
	return dec, nil
}

func splitProviders(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseBoolQuery(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func writeErr(status *int, response *[]byte, code int, err error) {
	*status = code
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	*response = b
}
