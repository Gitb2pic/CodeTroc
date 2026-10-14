//go:build js && wasm

package main

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"syscall/js"
	"time"
)

// jsConn est une connexion PostgreSQL dont l'autre bout est PGlite, appelé
// par la fonction JavaScript globale codetrocPg(Uint8Array) → Promise<Uint8Array>.
// Le côté JavaScript sérialise les appels : les réponses arrivent dans l'ordre.
type jsConn struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	err    error
	ready  chan struct{}
	closed bool
}

func newJSConn() *jsConn { return &jsConn{ready: make(chan struct{}, 1)} }

func (c *jsConn) signal() {
	select {
	case c.ready <- struct{}{}:
	default:
	}
}

func (c *jsConn) Write(p []byte) (int, error) {
	arr := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(arr, p)
	var onOK, onErr js.Func
	onOK = js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := args[0]
		out := make([]byte, data.Get("length").Int())
		js.CopyBytesToGo(out, data)
		c.mu.Lock()
		c.buf.Write(out)
		c.mu.Unlock()
		c.signal()
		onOK.Release()
		onErr.Release()
		return nil
	})
	onErr = js.FuncOf(func(_ js.Value, args []js.Value) any {
		c.mu.Lock()
		c.err = errors.New("pglite : " + args[0].Call("toString").String())
		c.mu.Unlock()
		c.signal()
		onOK.Release()
		onErr.Release()
		return nil
	})
	js.Global().Call("codetrocPg", arr).Call("then", onOK, onErr)
	return len(p), nil
}

func (c *jsConn) Read(p []byte) (int, error) {
	for {
		c.mu.Lock()
		if c.buf.Len() > 0 {
			n, _ := c.buf.Read(p)
			c.mu.Unlock()
			return n, nil
		}
		if c.err != nil || c.closed {
			err := c.err
			c.mu.Unlock()
			if err == nil {
				err = net.ErrClosed
			}
			return 0, err
		}
		c.mu.Unlock()
		<-c.ready
	}
}

func (c *jsConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.signal()
	return nil
}

type addr struct{}

func (addr) Network() string { return "pglite" }
func (addr) String() string  { return "pglite" }

func (c *jsConn) LocalAddr() net.Addr              { return addr{} }
func (c *jsConn) RemoteAddr() net.Addr             { return addr{} }
func (c *jsConn) SetDeadline(time.Time) error      { return nil }
func (c *jsConn) SetReadDeadline(time.Time) error  { return nil }
func (c *jsConn) SetWriteDeadline(time.Time) error { return nil }
