package main

import (
	"Tesseract/config"
	"Tesseract/lib/reflectStruct"
	"Tesseract/lib/settings"
	"Tesseract/pdf"
	"Tesseract/tesseract"
	"Tesseract/wrapper"
	"context"
	"log"
	"net/http"
	"sync"
)

var mux = http.NewServeMux()

func main() {

	wg := &sync.WaitGroup{}
	ctx, can := context.WithCancel(context.Background())
	defer can()

	cfg := &config.Config{}
	err := settings.JsonUnmarshalSettingsTo("HttpServer", cfg)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Handler: mux,
	}
	err = reflectStruct.CopyStruct(cfg, srv)
	if err != nil {
		log.Fatal(err)
	}

	pdfController := pdf.NewPDF(ctx, wg, 24)
	tessController, err := tesseract.NewTesseract()
	if err != nil {
		panic(err)
	}

	Pdf := wrapper.NewHTTPWrapper(pdfController.ServePDF)
	Tess := wrapper.NewHTTPWrapper(tessController.ServeTesseract)

	mux.HandleFunc("/PDF", Pdf.Serve)
	mux.HandleFunc("/Tesseract", Tess.Serve)

	// Sets http server to be shutdown last, len(endpoints)+1 because *http.Server is included
	log.Fatalln(srv.ListenAndServe())
}
