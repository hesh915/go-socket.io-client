package socketio_client

import (
	"github.com/zhouhui8915/engine.io-go/parser"
	"io"
	"sync"
)

type connReader struct {
	*parser.PacketDecoder
	closeChan chan struct{}
}

func newConnReader(d *parser.PacketDecoder, closeChan chan struct{}) *connReader {
	return &connReader{
		PacketDecoder: d,
		closeChan:     closeChan,
	}
}

func (r *connReader) Close() error {
	if r.closeChan == nil {
		return nil
	}
	r.closeChan <- struct{}{}
	r.closeChan = nil
	return nil
}

type connWriter struct {
	io.WriteCloser
	locker *sync.Mutex
}

func newConnWriter(w io.WriteCloser, locker *sync.Mutex) *connWriter {
	return &connWriter{
		WriteCloser: w,
		locker:      locker,
	}
}

func (w *connWriter) Close() error {
	defer func() {
		if w.locker != nil {
			w.locker.Unlock()
			w.locker = nil
		}
	}()
	return w.WriteCloser.Close()
}


// add with github.com/googollee/go-socket.io/ioutil.go

type writerHelper struct {
	writer io.Writer
	err    error
}

func newWriterHelper(w io.Writer) *writerHelper {
	return &writerHelper{
		writer: w,
	}
}

func (h *writerHelper) Write(p []byte) {
	if h.err != nil {
		return
	}
	for len(p) > 0 {
		n, err := h.writer.Write(p)
		if err != nil {
			h.err = err
			return
		}
		p = p[n:]
	}
}

func (h *writerHelper) Error() error {
	return h.err
}
