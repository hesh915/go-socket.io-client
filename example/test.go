package main

import (
	"github.com/zhouhui8915/go-socket.io-client"
	"log"
	"bufio"
	"os"
	"time"
)

func main() {

	opts := &socketio_client.Options{
		//Transport:"polling",
		Transport:"websocket",
		Query:make(map[string]string),
	}
	opts.Query["uid"] = "1"
	opts.Query["cid"] = "conf_123"
	uri := "http://192.168.1.70:9090"

	client,err := socketio_client.NewClient(uri,opts)
	if err != nil {
		log.Printf("NewClient error:%v\n",err)
		return
	}

	client.On("error", func() {
		log.Printf("on error\n")
	})
	client.On("connection", func() {
		log.Printf("on connect\n")
	})
	client.On("message", func(msg string) {
		log.Printf("on message:%v\n", msg)
	})
	client.On("disconnection", func() {
		log.Printf("on disconnect\n")
	})

	go func() {
		authStr := "{\"uid\":\"" + opts.Query["uid"] + "\",\"cid\":\"" + opts.Query["cid"] + "\"}"
		for {
			err := client.Emit("authenticate", authStr)
			if err != nil {
				log.Printf("Emit auth error:%v\n",err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	for  {
		data, _, _ := reader.ReadLine()
		command := string(data)
		err := client.Emit("message",command)
		if err != nil {
			log.Printf("Emit message error:%v\n",err)
			continue
		}
		log.Printf("send message:%v\n",command)
	}
}