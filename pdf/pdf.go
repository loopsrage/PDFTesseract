package pdf

import (
	"Tesseract/lib/configBase"
	"Tesseract/lib/gen/nodes"
	"Tesseract/lib/pdfpipeline/actionData"
	"Tesseract/lib/pdfpipeline/pdfium"
	"Tesseract/lib/pdfpipeline/pipeline"
	"Tesseract/lib/reflectStruct"
	"Tesseract/wrapper"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/uuid"
)

type DataWrapper struct {
	Data []byte `json:"Data"`
	Name string `json:"Name"`
}

type PDF struct {
	*configBase.FeatureController
	pl *pipeline.Queues
}

func (p *PDF) Source() string {
	return "PDFTesseract"
}

func (p *PDF) ServePDF(w http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	wg := &sync.WaitGroup{}
	var b []byte
	b, err := io.ReadAll(r.Body)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	dataWrap := &DataWrapper{}
	err = json.Unmarshal(b, &dataWrap)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	pc, err := pdfium.GetPageCount(dataWrap.Data)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	rid, err := uuid.NewUUID()
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}
	p.pl.Final.StoreResponse(rid.String(), make(chan interface{}, 1))
	for i := 0; i < pc; i++ {
		qd := nodes.NewQueueData(r.Context())
		md, err := reflectStruct.Map(&actionData.ActionData{
			Page:     i,
			Name:     strconv.Itoa(i),
			PDFbytes: dataWrap.Data,
		})
		if err != nil {
			*status = http.StatusBadRequest
			*response = []byte(err.Error())
			return
		}
		qd.SetAttributes(md)
		p.pl.PdfAction.SendSelf(qd)
	}
	wg.Wait()
	result := <-p.pl.Final.LoadResponse("")
	fmt.Println(result)
	rd, err := json.Marshal(result)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}
	*response = rd
}

func NewPDF(ctx context.Context, wg *sync.WaitGroup, maxInflight int64) *PDF {
	cf := &PDF{
		pl: pipeline.NewPipeline(ctx, wg, maxInflight),
	}
	ct := configBase.NewFeatureController(cf)
	ct.FeatureController = ct
	return cf
}
