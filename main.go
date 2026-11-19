package main

import (
	"context"
	"log"
	"net/http"
	"sync"

	"Tesseract/config"
	"Tesseract/handlers"
	"Tesseract/pdf"
	"Tesseract/tesseract"
	"Tesseract/wrapper"

	"github.com/loopsrage/gotools/lib/clsr"
	"github.com/loopsrage/gotools/lib/docpipeline"
	"github.com/loopsrage/gotools/lib/reflectStruct"
)

func main() {
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCfg := config.LoadAppConfig()

	engine, err := docpipeline.NewEngine(ctx, appCfg.EngineConfig(), nil, nil, nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	cc := clsr.NewCleanupComplete(cancel, wg)
	clsr.SigClosure(nil, cc, engine)

	mux := http.NewServeMux()
	api := handlers.NewAPI(engine)
	api.RegisterRoutes(mux)

	pdfController := pdf.NewPDF(engine)
	tessController := tesseract.NewTesseract(engine)
	mux.HandleFunc("/PDF", wrapper.NewHTTPWrapper(pdfController.ServePDF).Serve)
	mux.HandleFunc("/Tesseract", wrapper.NewHTTPWrapper(tessController.ServeTesseract).Serve)

	srv := &http.Server{Handler: mux}
	if err := reflectStruct.CopyStruct(&appCfg.Server, srv); err != nil {
		log.Fatal(err)
	}

	log.Printf("PDFTesseract listening on %s (PPS=%s, defaultOCR=%s)",
		srv.Addr, appCfg.PhaseServer.BaseURL, engine.OCRRegistry().DefaultName())
	log.Fatalln(srv.ListenAndServe())
}
