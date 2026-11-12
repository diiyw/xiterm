package terminal

const (
	ioQueueSize     = 64
	outputQueueSize = 16
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
	queue := ioQueueSize
	if size == outputBlockSize {
		queue = outputQueueSize
	}
	return bytePool{size: size, free: make(chan []byte, queue+2)}
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

// trim retains a small warm working set after the output burst ends. Channel
// operations allow the reader to borrow or return blocks concurrently.
func (p *bytePool) trim(keep int) {
	for len(p.free) > keep {
		select {
		case <-p.free:
		default:
			return
		}
	}
}

// TrimIdleBuffers is called by the UI only after the session has no queued
// output. It preserves incomplete escape sequences and a small I/O working set.
func (s *Session) TrimIdleBuffers() {
	if s.PendingOutput() {
		return
	}
	s.outputPool.trim(2)
	s.Term.TrimBuffers()
	if cap(s.findLine) > 4096 {
		s.findLine = nil
	}
	if cap(s.findColumns) > 4096 {
		s.findColumns = nil
	}
}
