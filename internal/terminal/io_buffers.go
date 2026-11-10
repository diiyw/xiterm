package terminal

const (
	ioQueueSize     = 64
	outputBlockSize = 32768
	inputBlockSize  = 1024
)

// bytePool keeps at most one block per queue slot, plus the producer and
// consumer blocks. Blocks are allocated lazily and survive GC, so a warmed
// transfer path does not allocate or box slices into interfaces.
type bytePool struct {
	size int
	free chan []byte
}

func newBytePool(size int) bytePool {
	return bytePool{size: size, free: make(chan []byte, ioQueueSize+2)}
}

func (p *bytePool) get() []byte {
	select {
	case b := <-p.free:
		return b
	default:
		return make([]byte, p.size)
	}
}

func (p *bytePool) put(b []byte) {
	if p.free == nil || cap(b) != p.size {
		return
	}
	select {
	case p.free <- b[:p.size]:
	default:
	}
}

func (p *bytePool) clear() {
	for {
		select {
		case <-p.free:
		default:
			return
		}
	}
}
