package concurr

import (
	"sync"
	"sync/atomic"
)

type TsList struct {
	count int64
	data  *sync.Map
}

func (t *TsList) Count() int64 {
	return atomic.LoadInt64(&t.count)
}

func (t *TsList) Add(x ...interface{}) *TsList {
	if len(x) == 0 {
		return t
	}

	var xCount int64
	sc := t.Count()
	for i := sc; i < sc+int64(len(x)); i++ {
		t.data.Store(i, x[xCount])
		atomic.AddInt64(&t.count, 1)
		xCount++
	}
	return t
}

func (t *TsList) At(position int64) interface{} {
	x, _ := t.data.Load(position)
	return x
}

func (t *TsList) All() (values []interface{}) {
	t.data.Range(func(_, value interface{}) bool {
		values = append(values, value)
		return true
	})
	return values
}

func NewTsList() *TsList {
	return &TsList{data: &sync.Map{}}
}
