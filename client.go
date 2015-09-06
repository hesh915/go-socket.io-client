package socketio_client

import (
	"reflect"
	"net/http"
	"net/url"
	"fmt"
)

var defaultTransport = "websocket"

type Options struct {
	Transport string //protocol name string,websocket polling...
	Query map[string]string //url的附加的参数

}

type Client struct {
	opts *Options

	socket *clientConn

	events map[string]*caller
	acks   map[int]*caller
}

func NewClient(uri string, opts *Options) (client *Client, err error) {
	exist := false
	for _, b := range transports {
		if b == opts.Transport {
			exist = true
		}
	}
	if !exist{
		opts.Transport = defaultTransport
	}

	request := &http.Request{}
	request.URL,err = url.Parse(uri)
	if err != nil {
		return
	}
	q:= request.URL.Query()
	for k,v := range opts.Query{
		q.Set(k,v)
	}
	request.URL.RawQuery = q.Encode()
	fmt.Println(request.URL.String())


	socket,err := newClientConn(opts.Transport,request)
	if err != nil {
		return
	}

	client = &Client{
		opts:opts,
		socket:socket,

		events:make(map[string]*caller),
		acks:make(map[int]*caller),
	}
	return
}

func (client *Client) On(message string, f interface{}) (err error) {
	c, err := newCaller(f)
	if err != nil {
		return
	}
	client.events[message] = c
	return
}

func (client *Client) Emit(message string, args ...interface{}) (err error) {
	var c *caller
	if l := len(args); l > 0 {
		fv := reflect.ValueOf(args[l-1])
		if fv.Kind() == reflect.Func {
			var err error
			c, err = newCaller(args[l-1])
			if err != nil {
				return err
			}
			args = args[:l-1]
		}
	}
	args = append([]interface{}{message}, args...)
	if c != nil {
		id, err := client.sendId(args)
		if err != nil {
			return err
		}
		client.acks[id] = c
		return nil
	}
	return client.send(args)
}

func (client *Client) sendId(args []interface{}) (int, error) {
	return 0,nil
}

func (client *Client) send(args []interface{}) error {
	return nil
}

func (client *Client) handshake(uri string)(err error){

	return
}

func (client *Client) handleMessage(msg []byte)(err error){

	return
}
