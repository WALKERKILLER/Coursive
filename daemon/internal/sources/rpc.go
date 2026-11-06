package sources

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// MaxMessage is the largest protocol message, in bytes, either way.
const MaxMessage = 1 << 20

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		Kind          string `json:"kind"`
		RetryAfterSec int    `json:"retryAfterSec"`
	} `json:"data,omitempty"`
}

// handler serves the provider's requests to the daemon (vault.*) and its
// notifications. A notification's result is discarded.
type handler func(method string, params json.RawMessage) (any, error)

// conn is one JSON-RPC session with a provider process.
type conn struct {
	w      io.Writer
	wmu    sync.Mutex
	next   atomic.Int64
	handle handler

	mu      sync.Mutex
	pending map[int64]chan message
	closed  chan struct{}
	err     error
}

func newConn(r io.Reader, w io.Writer, h handler) *conn {
	c := &conn{w: w, handle: h, pending: map[int64]chan message{}, closed: make(chan struct{})}
	go c.read(r)
	return c
}

func (c *conn) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), MaxMessage)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m.JSONRPC != "2.0" {
			c.close(errorf(KindUnavailable, "无法读取来源插件的回应：消息格式无效"))
			return
		}
		switch {
		case m.Method != "":
			go c.serve(m)
		case m.ID != nil:
			c.mu.Lock()
			ch := c.pending[*m.ID]
			delete(c.pending, *m.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
	err := sc.Err()
	if errors.Is(err, bufio.ErrTooLong) {
		c.close(errorf(KindUnavailable, "无法读取来源插件的回应：消息超过 1 MiB"))
		return
	}
	c.close(errorf(KindUnavailable, "来源插件已退出"))
}

func (c *conn) serve(m message) {
	result, err := c.handle(m.Method, m.Params)
	if m.ID == nil {
		return
	}
	out := message{JSONRPC: "2.0", ID: m.ID}
	if err != nil {
		out.Error = &rpcError{Code: -32000, Message: err.Error()}
	} else {
		out.Result, _ = json.Marshal(result)
	}
	c.write(out)
}

func (c *conn) write(m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > MaxMessage {
		return errorf(KindInvalid, "无法发送请求：消息超过 1 MiB")
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.w.Write(append(b, '\n'))
	return err
}

func (c *conn) close(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.closed:
		return
	default:
	}
	c.err = err
	close(c.closed)
}

// call sends one request and decodes its result into out (which may be nil).
func (c *conn) call(ctx context.Context, method string, params, out any) error {
	id := c.next.Add(1)
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	ch := make(chan message, 1)
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return c.err
	default:
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.write(message{JSONRPC: "2.0", ID: &id, Method: method, Params: raw}); err != nil {
		return errorf(KindUnavailable, "无法联系来源插件：%v", err)
	}
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errorf(KindUnavailable, "来源插件没有及时回应（%s）", method)
		}
		return ctx.Err()
	case <-c.closed:
		return c.err
	case m := <-ch:
		if m.Error != nil {
			e := &Error{Message: m.Error.Message, Kind: KindUnavailable}
			if m.Error.Data != nil {
				e.Kind, e.RetryAfterSec = m.Error.Data.Kind, m.Error.Data.RetryAfterSec
			}
			if e.Message == "" {
				e.Message = "无法完成来源插件的请求"
			}
			return e
		}
		if out == nil || len(m.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(m.Result, out); err != nil {
			return errorf(KindUnavailable, "无法读取来源插件的回应：%v", err)
		}
		return nil
	}
}
