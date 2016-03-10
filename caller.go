package socketio_client

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

type caller struct {
	sync.RWMutex
	Func reflect.Value
	Args []reflect.Type
}

func newCaller(f interface{}) (*caller, error) {
	fv := reflect.ValueOf(f)
	if fv.Kind() != reflect.Func {
		return nil, fmt.Errorf("f is not func")
	}
	ft := fv.Type()
	if ft.NumIn() == 0 {
		return &caller{
			Func: fv,
		}, nil
	}
	args := make([]reflect.Type, ft.NumIn())
	for i, n := 0, ft.NumIn(); i < n; i++ {
		args[i] = ft.In(i)
	}

	return &caller{
		Func: fv,
		Args: args,
	}, nil
}

func (c *caller) GetArgs() []interface{} {
	c.RLock()
	defer c.RUnlock()

	ret := make([]interface{}, len(c.Args))
	for i, argT := range c.Args {
		if argT.Kind() == reflect.Ptr {
			argT = argT.Elem()
		}
		v := reflect.New(argT)
		ret[i] = v.Interface()
	}
	return ret
}

func (c *caller) Call(args []interface{}) []reflect.Value {
	c.RLock()
	defer c.RUnlock()
	var a []reflect.Value
	diff := 0

	a = make([]reflect.Value, len(args))
	for i, arg := range args {
		v := reflect.ValueOf(arg)
		if c.Args[i].Kind() != reflect.Ptr {
			if v.IsValid() {
				v = v.Elem()
			} else {
				v = reflect.Zero(c.Args[i])
			}
		}
		a[i+diff] = v
	}

	if len(args) != len(c.Args) {
		return []reflect.Value{reflect.ValueOf([]interface{}{}), reflect.ValueOf(errors.New("Arguments do not match"))}
	}

	return c.Func.Call(a)
}
