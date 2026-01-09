package controller

import (
	"sync/atomic"
	"time"
)

type Controller struct {
	Tk       *time.Ticker
	Interval int64
}

func (c *Controller) Ticker() *time.Ticker {
	return c.Tk
}

func (c *Controller) Reset(proceed bool) {
	if proceed {
		c.Tk.Reset(1)
		return
	}
	c.Tk.Reset(time.Duration(atomic.LoadInt64(&c.Interval)))
}

func NewController(interval time.Duration, startNow bool) *Controller {
	tk := time.NewTicker(1)
	if !startNow {
		tk.Reset(interval)
	}

	return &Controller{
		Tk:       tk,
		Interval: int64(interval),
	}
}
