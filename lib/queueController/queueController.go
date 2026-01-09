package queueController

import (
	"Tesseract/lib/concurr"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type QueueController struct {
	ctx context.Context
	wg  *sync.WaitGroup
	wg2 *sync.WaitGroup

	queue     chan interface{}
	nextQueue chan interface{}
	prevQueue chan interface{}

	broadCast map[string]*QueueController

	wraps       concurr.Action
	handleError func(error) bool
}

func (q *QueueController) CheckContext() error {
	select {
	case <-q.ctx.Done():
		return q.ctx.Err()
	default:
		return nil
	}
}

func (q *QueueController) SetHandleError(handleErr func(err error) bool) {
	q.handleError = handleErr
}

func (q *QueueController) SetNext(queueController *QueueController) {
	q.nextQueue = queueController.queue
	queueController.prevQueue = q.nextQueue
}

func (q *QueueController) SendSelf(on interface{}) {
	q.wg2.Add(1)
	q.queue <- on
}

func (q *QueueController) SendNext(on interface{}) {
	if q.nextQueue == nil {
		q.wg2.Done()
		return
	}
	q.nextQueue <- on
}

func (q *QueueController) SetBroadcast(to map[string]*QueueController) {
	q.broadCast = to
}

func (q *QueueController) Broadcast(x QueueD) {
	for dr, qc := range q.broadCast {
		go func(der string, to *QueueController, y QueueD) {
			err := q.CheckContext()
			if err != nil {
				return
			}
			to.SendSelf(y.CopyDerivative(der))
		}(dr, qc, x)
	}
}

func (q *QueueController) Start(cfg *concurr.Config) {
	concurr.ActionQueue(q.ctx, q.wg, cfg, q.wraps, q.queue)
}

func (q *QueueController) HandleError(err error) bool {
	if err != nil {
		q.wg2.Done()
		q.handleError(errors.New(fmt.Sprintf("[%s] Command failed: %s. %+v", time.Now().Format(time.RFC3339), err.Error(), q.wraps)))
		return false
	}
	return true
}

func NewQueueController(ctx context.Context, wraps concurr.Action, wg *sync.WaitGroup) *QueueController {
	return &QueueController{
		ctx:   ctx,
		wg:    &sync.WaitGroup{},
		wg2:   wg,
		queue: make(chan interface{}, 1),
		wraps: wraps,
		handleError: func(err error) bool {
			fmt.Println(err)
			return true
		},
	}
}
