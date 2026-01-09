package asinGen

import (
	"Tesseract/lib/concurr"
	"context"
	"math/rand"
	"sync"
)

// character set for the trailing 8 characters (Uppercase A-Z and digits 0-9, excluding B and O to prevent repetition issues)
const charset = "ACDEFGHIJKLMNPQRSTUVWXYZ123456789"

// GenerateASIN generates a random 10-character alphanumeric ASIN starting with "B0".
func GenerateASIN() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset)-1)]
	}
	// ASINs typically start with B0
	return "B0" + string(b)
}

type AsinAction struct {
	Output *sync.Map
}

func (a *AsinAction) Action(on interface{}) {
	a.Output.Store(GenerateASIN(), struct{}{})
}

func NewAsinAction() *AsinAction {
	return &AsinAction{
		Output: &sync.Map{},
	}
}

func GenerateXUniqueAsin(ctx context.Context, cfg *concurr.Config, count uint64) *AsinAction {
	as := NewAsinAction()
	var ids []interface{}
	for i := 0; i < int(count); i++ {
		ids = append(ids, i)
	}
	concurr.ActionOn(ctx, cfg, as, ids...)
	return as
}
