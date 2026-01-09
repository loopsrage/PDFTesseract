package asinGen

import (
	"Tesseract/lib/concurr"
	"bytes"
	"context"
	"os"
	"testing"
)

func TestGenerateAsins(t *testing.T) {
	buff := bytes.NewBuffer(nil)
	t.Run("Generate ASIN", func(t *testing.T) {
		asins := GenerateXUniqueAsin(context.Background(), &concurr.Config{
			Max: 360,
		}, 100000)
		asins.Output.Range(func(key, _ any) bool {
			switch key := key.(type) {
			case string:
				buff.Write([]byte(key + "\n"))
			}
			return true
		})
	})

	err := os.WriteFile("./training_text.txt", buff.Bytes(), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
}
