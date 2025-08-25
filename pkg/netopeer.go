package netopeer

import (
	"context"
	"encoding/xml"
	"fmt"
	"sync"
	"sync/atomic"
)

type Cb func([]byte, error)

var (
	sr = []byte(`<create-subscription xmlns="urn:ietf:params:xml:ns:netconf:notification:1.0"/>`)
)

func NewNetopeer(ctx context.Context, unix string) (np *Netopeer, err error) {
	np = &Netopeer{
		ch:   make(chan Resp, 10),
		reqs: make(map[int64]Cb),
	}
	np.sock, err = NewUsock(context.Background(), unix, np.ch)
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

func (np *Netopeer) Subscribe(cb Cb) {
	func() {
		np.mu.Lock()
		defer np.mu.Unlock()

		np.subs = cb
	}()
	go func() {
		idx := atomic.AddInt64(&np.seq, 1)

		req := &Rpc{
			Xmlns:     "urn:ietf:params:xml:ns:netconf:base:1.0", // fixme!
			MessageID: idx,
			Data:      string(sr)}
		np.sock.Send(req.Marshal(), false)
	}()
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

					nf := &Notification{}
					if err := xml.Unmarshal(r.Body, nf); err != nil {
						np.Println("notif unmarshal err", err)
						// todo.. hello is also here
						continue
					}
					if np.subs != nil {
						if nf.NetconfRpcExecution != nil {
							np.subs(nf.NetconfRpcExecution.Data, nil)
						}
						if nf.NetconfConfigChange != nil {
							np.subs(nf.NetconfConfigChange.Data, nil)
						}
					}
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
