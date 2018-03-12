package pool

type BytePool struct {
	list       chan []byte
	bufferSize int
}

func NewBytePool(poolSize, bufferSize int) *BytePool {
	b := &BytePool{
		list:       make(chan []byte, poolSize),
		bufferSize: bufferSize,
	}

	return b
}

func (p *BytePool) Put(b []byte) {
	select {
	case p.list <- b:
	default:
	}
}

func (p *BytePool) Get() []byte {
	select {
	case b := <-p.list:
		return b
	default:
		return make([]byte, p.bufferSize)
	}

	return nil
}

func (p *BytePool) BufferSize() int {
	return p.bufferSize
}
