package pool

import (
	"bytes"
)

type BufferPool struct {
	list chan *bytes.Buffer
}

func NewBufferPool(poolSize int) *BufferPool {
	b := &BufferPool{
		list: make(chan *bytes.Buffer, poolSize),
	}

	return b
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	select {
	case p.list <- b:
	default:
	}
}

func (p *BufferPool) Get() *bytes.Buffer {
	select {
	case b := <-p.list:
		return b
	default:
		return &bytes.Buffer{}
	}

	return nil
}
