package tesseract

import (
	"Tesseract/lib/clsr"
	"Tesseract/lib/gocvlib/effects"
	"Tesseract/lib/gocvlib/gocvf"
	"Tesseract/lib/gosseractlib"
	"Tesseract/wrapper"
	"errors"
	"image/color"
	"log"

	"github.com/otiai10/gosseract/v2"
	"gocv.io/x/gocv"

	"io"
	"net/http"

	_ "image/jpeg"
	_ "image/png"
)

type DataWrapper struct {
	Data []byte `json:"Data"`
	Name string `json:"Name"`
}

type Tesseract struct {
	client *gosseract.Client
}

func (t *Tesseract) ServeTesseract(w http.ResponseWriter, r *http.Request) {
	var (
		status   = new(int)
		response = new([]byte)
	)
	*status = http.StatusOK
	defer wrapper.WriteResponseHeaders(r, status, response)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	img, err := gocv.IMDecode(b, gocv.IMReadAnyColor)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}
	defer img.Close()

	if img.Empty() {
		*status = http.StatusBadRequest
		*response = []byte(errors.New("empty image").Error())
		return
	}

	cp := gocv.NewMat()
	defer cp.Close()

	err = img.CopyTo(&cp)
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	ec := effects.NewEffectChain()
	out, err := ec.Chain(&cp, effects.WordIsolationChain())
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}
	defer out.Close()

	if out == nil {
		*status = http.StatusBadRequest
		*response = []byte("nil output")
		return
	}

	result := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC3)
	defer result.Close()

	rects := gocvf.FindContours(out, 1500, 10000, gocv.RetrievalTree, gocv.ChainApproxTC89L1)
	for _, rect := range rects {
		err = gocv.Rectangle(&result, rect, color.RGBA{
			R: 0,
			G: 255,
			B: 0,
		}, 1)
		if err != nil {
			return
		}
	}

	//tfc := rectAction.ExtractTextFromCellsFromRects(context.Background(), concurr.NewConfig(50), &img, rects)

	//fmt.Printf("%+v\n", tfc)

	buffer, err := gocv.IMEncode(".png", img)
	if err != nil {
		log.Fatalf("Error encoding Mat to bytes: %v", err)
	}
	defer buffer.Close()

	err = t.client.SetImageFromBytes(buffer.GetBytes())
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}

	output, err := t.client.Text()
	if err != nil {
		*status = http.StatusBadRequest
		*response = []byte(err.Error())
		return
	}
	*response = []byte(output)
}

func NewTesseract() (*Tesseract, error) {
	client, err := gosseractlib.GosseractClient("/home/jarek/tess/tesseract/tessdata", "AmazonEmber")
	if err != nil {
		return nil, err
	}

	clsr.SigClosure(nil, client)
	return &Tesseract{client: client}, nil
}
