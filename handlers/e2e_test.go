package handlers_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"Tesseract/config"
	"Tesseract/handlers"
	"Tesseract/pdf"
	"Tesseract/tesseract"
	"Tesseract/wrapper"

	"github.com/loopsrage/go_mltools/ppsclient"
	"github.com/loopsrage/gotools/lib/docpipeline"
	"github.com/loopsrage/gotools/lib/visionlib"
)

func TestLiveProceduralPhaseServerE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	appCfg := config.DefaultAppConfig()
	appCfg.PhaseServer.BaseURL = "http://127.0.0.1:30180"
	appCfg.PhaseServer.RootTopic = fmt.Sprintf("ocr.e2e_%d", time.Now().UnixNano())
	appCfg.PhaseServer.Collection = fmt.Sprintf("ocr_e2e_%d", time.Now().UnixNano())

	probeClient := ppsclient.NewClient(appCfg.PhaseServer)
	st, err := probeClient.Status(ctx)
	if err != nil {
		t.Fatalf("live Minikube proceduralphaseserver unreachable at %s: %v", appCfg.PhaseServer.BaseURL, err)
	}
	if st["status"] != "ok" {
		t.Fatalf("unexpected proceduralphaseserver status: %+v", st)
	}

	pdfRenderer := visionlib.NewPDFRenderer(2)
	defer pdfRenderer.Close()

	eng, err := docpipeline.NewEngine(ctx, appCfg.EngineConfig(), nil, nil, pdfRenderer, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	mux := http.NewServeMux()
	api := handlers.NewAPI(eng)
	api.RegisterRoutes(mux)
	mux.HandleFunc("/PDF", wrapper.NewHTTPWrapper(pdf.NewPDF(eng).ServePDF).Serve)
	mux.HandleFunc("/Tesseract", wrapper.NewHTTPWrapper(tesseract.NewTesseract(eng).ServeTesseract).Serve)

	appSrv := httptest.NewServer(mux)
	defer appSrv.Close()

	altOCRSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		imgB64, _ := req["image_b64"].(string)
		raw, err := base64.StdEncoding.DecodeString(imgB64)
		if err != nil || len(raw) == 0 {
			http.Error(w, "invalid image_b64", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model_id":   "alt-vision-ocr-v2",
			"text":       "ALT_MODEL_VERIFIED",
			"confidence": 0.97,
		})
	}))
	defer altOCRSrv.Close()

	regBody, _ := json.Marshal(map[string]any{
		"name":        "alt_ocr",
		"endpoint":    altOCRSrv.URL,
		"model_id":    "alt-vision-ocr-v2",
		"set_default": false,
	})
	regResp, err := http.Post(appSrv.URL+"/Providers/HTTP", "application/json", bytes.NewReader(regBody))
	if err != nil || regResp.StatusCode != http.StatusOK {
		t.Fatalf("register HTTP provider failed: resp=%v err=%v", regResp, err)
	}
	regResp.Body.Close()

	// 1. Direct Image Upload -> Unified Post-Render Pipeline (tesseract + alt_ocr)
	tmpPDF := visionlib.BuildMultiPagePDF([]string{"DIRECT IMAGE 7391"})
	renderedImg, err := pdfRenderer.RenderPage(tmpPDF, 0, 200)
	if err != nil {
		t.Fatalf("RenderPage for direct image fixture: %v", err)
	}

	imgReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		appSrv.URL+"/Image?name=scan_7391.png&ocr_providers=tesseract,alt_ocr",
		bytes.NewReader(renderedImg.PNGBytes),
	)
	if err != nil {
		t.Fatalf("NewRequest /Image: %v", err)
	}
	imgReq.Header.Set("Content-Type", "image/png")

	imgHTTPResp, err := http.DefaultClient.Do(imgReq)
	if err != nil {
		t.Fatalf("POST /Image: %v", err)
	}
	defer imgHTTPResp.Body.Close()
	if imgHTTPResp.StatusCode != http.StatusOK {
		var errMap map[string]any
		_ = json.NewDecoder(imgHTTPResp.Body).Decode(&errMap)
		t.Fatalf("POST /Image HTTP %d: %+v", imgHTTPResp.StatusCode, errMap)
	}

	var imgRes docpipeline.ProcessResult
	if err := json.NewDecoder(imgHTTPResp.Body).Decode(&imgRes); err != nil {
		t.Fatalf("decode /Image ProcessResult: %v", err)
	}
	if imgRes.SourceKind != "image" || imgRes.PageCount != 1 {
		t.Fatalf("unexpected image result: %+v", imgRes)
	}
	if !strings.Contains(strings.ToUpper(imgRes.FullText), "DIRECT") || !strings.Contains(imgRes.FullText, "7391") {
		t.Fatalf("direct image OCR FullText = %q, want DIRECT IMAGE 7391", imgRes.FullText)
	}
	if imgRes.Pages[0].Summary["alt_ocr"] == nil || !strings.Contains(imgRes.Pages[0].Summary["alt_ocr"].FullText, "ALT_MODEL_VERIFIED") {
		t.Fatalf("expected alt_ocr output alongside tesseract, got: %+v", imgRes.Pages[0].Summary)
	}

	hierResp, err := http.Get(appSrv.URL + "/Documents/" + imgRes.DocID + "?include_blobs=true")
	if err != nil {
		t.Fatalf("GET /Documents/%s: %v", imgRes.DocID, err)
	}
	defer hierResp.Body.Close()
	if hierResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /Documents/%s HTTP %d", imgRes.DocID, hierResp.StatusCode)
	}

	var docTree ppsclient.DocumentHierarchyView
	if err := json.NewDecoder(hierResp.Body).Decode(&docTree); err != nil {
		t.Fatalf("decode DocumentHierarchyView: %v", err)
	}
	if docTree.Manifest["status"] != "completed" {
		t.Fatalf("manifest status = %v, want completed", docTree.Manifest["status"])
	}
	recoveredImgBytes, err := ppsclient.ReassembleSourceBytes(docTree.Source, nil)
	if err != nil || !bytes.Equal(recoveredImgBytes, renderedImg.PNGBytes) {
		t.Fatalf("source Base64 round-trip failed: err=%v len=%d want=%d", err, len(recoveredImgBytes), len(renderedImg.PNGBytes))
	}
	if len(docTree.Pages) != 1 || len(docTree.Pages[0].Regions) == 0 {
		t.Fatalf("expected 1 page with regions in hierarchy, got: %+v", docTree.Pages)
	}
	reg0 := docTree.Pages[0].Regions[0]
	if reg0.Images["crop_denoised"] == nil || reg0.OCR["tesseract"] == nil || reg0.OCR["alt_ocr"] == nil {
		t.Fatalf("missing region hierarchy artifacts: images=%v ocr=%v", reg0.Images, reg0.OCR)
	}
	if docTree.Audit == nil || docTree.Audit["head_hash"] == "" {
		t.Fatalf("missing SHA-256 audit chain in proceduralphaseserver: %+v", docTree.Audit)
	}

	// 2. Multi-Page PDF Upload via /PDF + Concurrent indexQueue Load + .pt Flush
	pdfBytes := visionlib.BuildMultiPagePDF([]string{"LEDGER PAGE ONE", "BALANCE 9420"})
	pdfJSON, _ := json.Marshal(map[string]any{
		"Data":    base64.StdEncoding.EncodeToString(pdfBytes),
		"Name":    "ledger_2p.pdf",
		"DPI":     150,
		"FlushPT": true,
	})

	var concWG sync.WaitGroup
	errs := make(chan error, 3)
	var primaryPDFRes *docpipeline.ProcessResult
	var resMu sync.Mutex

	for i := 0; i < 3; i++ {
		concWG.Add(1)
		go func(idx int) {
			defer concWG.Done()
			u := fmt.Sprintf("%s/PDF?doc_id=ledger-conc-%d-%d", appSrv.URL, time.Now().UnixNano(), idx)
			pReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(pdfJSON))
			if err != nil {
				errs <- err
				return
			}
			pReq.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(pReq)
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("concurrent /PDF[%d] HTTP %d", idx, resp.StatusCode)
				return
			}
			var pr docpipeline.ProcessResult
			if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
				errs <- err
				return
			}
			if pr.PageCount != 2 || !strings.Contains(strings.ToUpper(pr.FullText), "LEDGER") || !strings.Contains(pr.FullText, "9420") {
				errs <- fmt.Errorf("concurrent /PDF[%d] unexpected OCR result: pages=%d text=%q", idx, pr.PageCount, pr.FullText)
				return
			}
			resMu.Lock()
			if primaryPDFRes == nil {
				primaryPDFRes = &pr
			}
			resMu.Unlock()
		}(i)
	}
	concWG.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	if primaryPDFRes == nil {
		t.Fatal("missing primaryPDFRes")
	}

	healthResp, err := http.Get(appSrv.URL + "/Health")
	if err != nil || healthResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /Health failed: %v", err)
	}
	defer healthResp.Body.Close()
	var healthData map[string]any
	_ = json.NewDecoder(healthResp.Body).Decode(&healthData)
	plStats, _ := healthData["pipeline"].(map[string]any)
	if inFlight, _ := plStats["queue_total_in_flight"].(float64); inFlight != 0 {
		t.Fatalf("queue_total_in_flight = %v, want 0", inFlight)
	}
	ppsState, _ := healthData["pps"].(map[string]any)
	if ppsState["pt_file_exists"] != true {
		t.Fatalf("expected pt_file_exists=true on Minikube proceduralphaseserver, got: %+v", ppsState)
	}
}
