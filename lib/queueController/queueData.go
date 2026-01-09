package queueController

import (
	"Tesseract/lib/featureSettings"
	"Tesseract/lib/index"
	"sync/atomic"
)

type QueueData struct {
	Derivitive *atomic.Value
	Data       *index.Index
	*featureSettings.Config
}

func (c *QueueData) CopyDerivative(derivative string) QueueD {
	der := &atomic.Value{}
	der.Store(derivative)

	nqd := &QueueData{
		Derivitive: der,
	}
	nqd.Config = c.Config
	nqd.Data = c.Data

	return nqd
}

type QueueD interface {
	Derivative() string
	CopyDerivative(derivative string) QueueD
}

func (c *QueueData) Derivative() string {
	if c.Derivitive == nil {
		return ""
	}

	switch d := c.Derivitive.Load().(type) {
	case string:
		return d
	default:
		return ""
	}
}

func (c *QueueData) SetAttributes(attributes map[string]interface{}) {
	for k, v := range attributes {
		c.SetAttribute(k, v)
	}
}

func NewQueueData() *QueueData {
	return &QueueData{
		Derivitive: nil,
		Data:       index.NewIndex(),
		Config:     featureSettings.NewConfig(),
	}
}
