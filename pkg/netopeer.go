package netopeer

import (
	"context"
	"encoding/xml"
	"fmt"
	"sync"
	"sync/atomic"
)

type Cb func([]byte, error)

func NewNetopeer(ctx context.Context, unix string) (np *Netopeer, err error) {
	np = &Netopeer{
		ch:   make(chan Resp, 10),
		reqs: make(map[int64]Cb),
	}
	np.sock, err = NewUsock(context.Background(), TestSock, np.ch)
	if err != nil {
		return
	}

	np.sock.Send([]byte(Hello), true)

	go np.run(ctx)
	return
}

type Netopeer struct {
	mu   sync.Mutex
	ch   chan Resp
	seq  int64
	sock *Usock

	subs Cb
	reqs map[int64]Cb
}

func (np *Netopeer) Request(data []byte, cb Cb) {

	idx := atomic.AddInt64(&np.seq, 1)

	go func() {
		req := &Rpc{
			Xmlns:     "urn:ietf:params:xml:ns:netconf:base:1.0", // fixme!
			MessageID: idx,
			Data:      string(data)}
		np.storeCb(idx, cb)
		np.sock.Send(req.Marshal(), false)
	}()
}

func (np *Netopeer) Subscribe(data interface{}, cb Cb) {

}

func (np *Netopeer) storeCb(idx int64, cb Cb) {
	np.mu.Lock()
	defer np.mu.Unlock()

	np.reqs[idx] = cb
}

func (np *Netopeer) chkAndRemove(idx int64) Cb {
	np.mu.Lock()
	defer np.mu.Unlock()

	cb, ok := np.reqs[idx]
	if ok {
		delete(np.reqs, idx)
	}
	return cb
}

func (np *Netopeer) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-np.ch:
			if r.Err != nil {
				np.Println("rx err", r.Err)
				// todo: process somehow, terminate requests for app, restart usock etc
			} else {
				rr := &RpcReply{}
				if err := xml.Unmarshal(r.Body, rr); err != nil {
					// np.Println("rx unmarshal err", err, "->", string(r.Body))
					// todo.. hello is also here
					continue
				}
				cb := np.chkAndRemove(rr.MessageID)
				if cb != nil {
					cb(rr.Data.Data, nil)
				}
			}
		}
	}
}

func (np *Netopeer) Println(i ...interface{}) {
	fmt.Println("np2:", i)
}
