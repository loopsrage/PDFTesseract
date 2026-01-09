package pipelines

import (
	"Tesseract/lib/asinGen"
	"Tesseract/lib/concurr"
	"Tesseract/lib/gen/nodes"
	"Tesseract/lib/gen/util"
	"Tesseract/lib/reflectStruct"
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"testing"
	"time"

	"gocv.io/x/gocv"
)
import _ "net/http/pprof"

func TestPipeline(t *testing.T) {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	wg := &sync.WaitGroup{}

	img := gocv.IMRead("/home/jarek/GolandProjects/Tesseract/lib/gen/Image_2025_10_28_0001.png", gocv.IMReadColor)
	if img.Empty() {
		t.Error("Error IMReadColor")
		return
	}
	defer img.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := concurr.NewConfig(1024)
	st := time.Now()
	pl := NewPipeline(ctx, cfg, wg)

	nf, err := NoiseFunctions(ctx, cfg, &img, wg, pl.LoadNode(nodes.GenerateLineBoxNode.String()))
	if err != nil {
		t.Fatal(err)
	}
	pl.LoadNode(nodes.Text2ImageNode.String()).SetBroadcast(nf)
	// Start an HTTP server for profiling (can run in a separate goroutine)

	node := pl.LoadNode(nodes.Text2ImageNode.String())
	if node == nil {
		panic("Error LoadNode")
	}

	const iterations = 10000
	const flushInterval = 500
	wakeupInterval := time.Minute * 4
	for i := 0; i < iterations; i++ {
		qd := nodes.NewQueueData(ctx)
		opm, err := reflectStruct.Map(&nodes.Args{
			GeneratedAsin: asinGen.GenerateASIN(),
			OutputBase:    "/home/jarek/tess/tesstrain/data/AmazonEmber-ground-truth",
			FontName:      "Amazon Ember",
			FontDir:       "/usr/share/fonts",
		})
		if err != nil {
			panic(err)
		}
		qd.SetAttributes(opm)
		node.SendSelf(qd)
		if i%flushInterval == 0 && i > 0 {
			fmt.Printf("Processing %d iterations. Sleeping for %s minutes and dropping caches\n", flushInterval, wakeupInterval)
			wg.Wait()
			cmd := exec.Command("sudo", "sh", "-c", "echo 3 > /proc/sys/vm/drop_caches")
			resp, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("Success", string(resp))
			time.Sleep(wakeupInterval / 2)
			log.Println("Waking up, resuming pipeline.")
		}
	}
	wg.Wait()

	for k, v := range nf {
		fmt.Println(k, v)
	}

	err = pl.idx.RangeIndex("Nodes", func(key, value interface{}) {
		fmt.Println(key, value.(*nodes.QueueActionWrapper).Count())
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("[%s] Duration: %f", time.Now().Format(time.RFC3339), time.Since(st).Seconds())
}

func TestMakeTrainging(t *testing.T) {
	t.Run("test MakeTrainging", func(t *testing.T) {
		ta := &util.MakeAction{Ctx: context.Background()}
		err := ta.MakeTraining()
		if err != nil {
			t.Fatal(err)
		}
	})
}
