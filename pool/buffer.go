package pool

import (
	"bytes"
	"sync"
)

type BufferPool struct {
	p sync.Pool
}

// Get returns a buffer from the pool
func (p *BufferPool) Get() *bytes.Buffer {
	return p.p.Get().(*bytes.Buffer)
}

// Put resets the buffer and puts it back to the pool
func (p *BufferPool) Put(v *bytes.Buffer) {
	v.Reset()
	p.p.Put(v)
}

func NewBufferPool(defaultSize int) *BufferPool {
	return &BufferPool{
		p: sync.Pool{
			New: func() interface{} {
				if defaultSize == 0 {
					return &bytes.Buffer{}
				}

				return bytes.NewBuffer(make([]byte, 0, defaultSize))
			},
		},
	}
}
