package socketio_client

import (
	"errors"
	"fmt"
	"github.com/googollee/go-engine.io/message"
	"github.com/googollee/go-engine.io/parser"
	"github.com/googollee/go-engine.io/polling"
	"github.com/googollee/go-engine.io/transport"
	"github.com/googollee/go-engine.io/websocket"
	"io"
	"net/http"
	"sync"
	"time"
)

var InvalidError = errors.New("invalid transport")

var transports = []string{"polling", "websocket"}

var creaters map[string]transport.Creater

func init() {
	creaters = make(map[string]transport.Creater)

	for _, t := range transports {
		switch t {
		case "polling":
			creaters[t] = polling.Creater
		case "websocket":
			creaters[t] = websocket.Creater
		}
	}
}

type MessageType message.MessageType

const (
	MessageBinary MessageType = MessageType(message.MessageBinary)
	MessageText   MessageType = MessageType(message.MessageText)
)

type state int

const (
	stateUnknow state = iota
	stateNormal
	stateUpgrading
	stateClosing
	stateClosed
)

type clientConn struct {
	id              string
	transportName   string
	request         *http.Request
	writerLocker    sync.Mutex
	transportLocker sync.RWMutex
	currentName     string
	current         transport.Client
	upgradingName   string
	upgrading       transport.Client
	state           state
	stateLocker     sync.RWMutex
	readerChan      chan *connReader
	pingTimeout     time.Duration
	pingInterval    time.Duration
	pingChan        chan bool
}

func newClientConn(transportName string, r *http.Request) (client *clientConn, err error) {
	if transportName == "" {
		transportName = "polling"
	}

	client = &clientConn{
		request:       r,
		transportName: transportName,
		state:         stateNormal,
		pingTimeout:   60000 * time.Millisecond,
		pingInterval:  25000 * time.Millisecond,
		pingChan:      make(chan bool),
		readerChan:    make(chan *connReader),
	}

	err = client.onOpen()
	if err != nil {
		return
	}

	go client.pingLoop()

	return
}

func (c *clientConn) Id() string {
	return c.id
}

func (c *clientConn) Request() *http.Request {
	return c.request
}

func (c *clientConn) NextReader() (MessageType, io.ReadCloser, error) {
	if c.getState() == stateClosed {
		return MessageBinary, nil, io.EOF
	}
	ret := <-c.readerChan
	if ret == nil {
		return MessageBinary, nil, io.EOF
	}
	return MessageType(ret.MessageType()), ret, nil
}

func (c *clientConn) NextWriter(t MessageType) (io.WriteCloser, error) {
	switch c.getState() {
	case stateUpgrading:
		for i := 0; i < 30; i++ {
			time.Sleep(50 * time.Millisecond)
			if c.getState() != stateUpgrading {
				break
			}
		}
		if c.getState() == stateUpgrading {
			return nil, fmt.Errorf("upgrading")
		}
	case stateNormal:
	default:
		return nil, io.EOF
	}
	ret, err := c.getCurrent().NextWriter(message.MessageType(t), parser.MESSAGE)
	return ret, err
}

func (c *clientConn) Close() error {
	if c.getState() != stateNormal && c.getState() != stateUpgrading {
		return nil
	}
	if c.upgrading != nil {
		c.upgrading.Close()
	}
	c.writerLocker.Lock()
	if w, err := c.getCurrent().NextWriter(message.MessageText, parser.CLOSE); err == nil {
		writer := newConnWriter(w, &c.writerLocker)
		writer.Close()
	} else {
		c.writerLocker.Unlock()
	}
	if err := c.getCurrent().Close(); err != nil {
		return err
	}
	c.setState(stateClosing)
	return nil
}

func (c *clientConn) OnPacket(r *parser.PacketDecoder) {
	if s := c.getState(); s != stateNormal && s != stateUpgrading {
		return
	}
	switch r.Type() {
	case parser.OPEN:
	case parser.CLOSE:
		c.getCurrent().Close()
	case parser.PING:
		t := c.getCurrent()
		u := c.getUpgrade()
		newWriter := t.NextWriter
		if u != nil {
			if w, _ := t.NextWriter(message.MessageText, parser.NOOP); w != nil {
				w.Close()
			}
			newWriter = u.NextWriter
		}
		if w, _ := newWriter(message.MessageText, parser.PONG); w != nil {
			io.Copy(w, r)
			w.Close()
		}
		fallthrough
	case parser.PONG:
		c.pingChan <- true
	case parser.MESSAGE:
		closeChan := make(chan struct{})
		c.readerChan <- newConnReader(r, closeChan)
		<-closeChan
		close(closeChan)
		r.Close()
	case parser.UPGRADE:
		c.upgraded()
	case parser.NOOP:
	}
}

func (c *clientConn) OnClose(server transport.Client) {
	if t := c.getUpgrade(); server == t {
		c.setUpgrading("", nil)
		t.Close()
		return
	}
	t := c.getCurrent()
	if server != t {
		return
	}
	t.Close()
	if t := c.getUpgrade(); t != nil {
		t.Close()
		c.setUpgrading("", nil)
	}
	c.setState(stateClosed)
	close(c.readerChan)
	close(c.pingChan)
}

func (c *clientConn) onOpen() error {

	creater, exists := creaters["polling"]
	if !exists {
		return InvalidError
	}

	q := c.request.URL.Query()
	q.Set("transport", "polling")
	c.request.URL.RawQuery = q.Encode()

	transport, err := creater.Client(c.request)
	if err != nil {
		return err
	}
	c.setCurrent("polling", transport)

	pack, err := c.getCurrent().NextReader()
	if err != nil {
		return err
	}
	fmt.Println(pack)

	//var p []byte
	p := make([]byte, 1024)
	l, err := pack.Read(p)
	fmt.Println(l)
	fmt.Println(err)
	fmt.Println(string(p))

	creater, exists = creaters["websocket"]
	if !exists {
		return InvalidError
	}

	c.request.URL.Scheme = "ws"
	q.Set("sid", "0")
	q.Set("transport", "websocket")
	c.request.URL.RawQuery = q.Encode()

	transport, err = creater.Client(c.request)
	if err != nil {
		return err
	}
	c.setCurrent("websocket", transport)

	pack, err = c.getCurrent().NextReader()
	if err != nil {
		return err
	}
	fmt.Println(pack)

	p2 := make([]byte, 1024)
	l, err = pack.Read(p2)
	fmt.Println(l)
	fmt.Println(err)
	fmt.Println(string(p2))

	fmt.Println("end")

	return nil
}

func (c *clientConn) getCurrent() transport.Client {
	c.transportLocker.RLock()
	defer c.transportLocker.RUnlock()

	return c.current
}

func (c *clientConn) getUpgrade() transport.Client {
	c.transportLocker.RLock()
	defer c.transportLocker.RUnlock()

	return c.upgrading
}

func (c *clientConn) setCurrent(name string, s transport.Client) {
	c.transportLocker.Lock()
	defer c.transportLocker.Unlock()

	c.currentName = name
	c.current = s
}

func (c *clientConn) setUpgrading(name string, s transport.Client) {
	c.transportLocker.Lock()
	defer c.transportLocker.Unlock()

	c.upgradingName = name
	c.upgrading = s
	c.setState(stateUpgrading)
}

func (c *clientConn) upgraded() {
	c.transportLocker.Lock()

	current := c.current
	c.current = c.upgrading
	c.currentName = c.upgradingName
	c.upgrading = nil
	c.upgradingName = ""

	c.transportLocker.Unlock()

	current.Close()
	c.setState(stateNormal)
}

func (c *clientConn) getState() state {
	c.stateLocker.RLock()
	defer c.stateLocker.RUnlock()
	return c.state
}

func (c *clientConn) setState(state state) {
	c.stateLocker.Lock()
	defer c.stateLocker.Unlock()
	c.state = state
}

func (c *clientConn) pingLoop() {
	lastPing := time.Now()
	lastTry := lastPing
	for {
		now := time.Now()
		pingDiff := now.Sub(lastPing)
		tryDiff := now.Sub(lastTry)
		select {
		case ok := <-c.pingChan:
			if !ok {
				return
			}
			lastPing = time.Now()
			lastTry = lastPing
		case <-time.After(c.pingInterval - tryDiff):
			c.writerLocker.Lock()
			if w, _ := c.getCurrent().NextWriter(message.MessageText, parser.PING); w != nil {
				writer := newConnWriter(w, &c.writerLocker)
				writer.Close()
			} else {
				c.writerLocker.Unlock()
			}
			lastTry = time.Now()
		case <-time.After(c.pingTimeout - pingDiff):
			c.Close()
			return
		}
	}
}
