package netopeer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

const (
	NetopeerSock = "/tmp/netopeer2.sock"
	TestSock     = "/tmp/proxy.sock"

	hello_tail = "]]>]]>"
	split      = 1024

	// fixme: this should be created at the upper layer
	Hello = `<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"><capabilities><capability>urn:ietf:params:netconf:base:1.0</capability><capability>urn:ietf:params:netconf:base:1.1</capability></capabilities></hello>`
)

var (
	ErrUnexpected = errors.New("error: unexpected data")

	magicPrefix = []byte{10, 35} // \n#
	lastSuffix  = []byte{35, 10}
)

type Resp struct {
	Body []byte
	Err  error
}

func NewUsock(ctx context.Context, unix string, resp chan Resp) (*Usock, error) {
	u := &Usock{resp: resp}
	c, cf := context.WithCancel(ctx)
	u.cf = cf

	var err error
	if u.conn, err = net.Dial("unix", unix); err != nil {
		return nil, err
	}

	go u.run(c)

	return u, nil
}

func (u *Usock) notify(b []byte, err error) {
	r := Resp{
		Body: b,
		Err:  err,
	}
	u.resp <- r
	if err != nil {
		u.cf()
	}
}

func (u *Usock) run(ctx context.Context) {
	defer u.conn.Close()

	b := make([]byte, 4096)
	magic := make([]byte, 2)

	// expecting hello

	_, err := u.conn.Read(b)
	if err != nil {
		u.Println("rd err:", err)
		u.notify(b, err)
		return
	}
	s := string(b)
	offs := strings.Index(s, hello_tail)
	if offs < 0 {
		u.Println("no hello tail", s)
		u.notify(b, err)
		return
	}
	u.notify(b[:offs], nil)

	// reading regular data
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := u.conn.Read(magic)
			if n != len(magic) || err != nil {
				u.Println("can't read magic")
				u.notify(nil, ErrUnexpected)
				return
			}
			if !bytes.Contains(magic, magicPrefix) {
				u.Println("no magic", magic, string(magic))
				u.notify(nil, ErrUnexpected)
				return
			}

			// expecting either tail or digits
			n, err = u.conn.Read(magic)
			if n != len(magic) || err != nil {
				u.Println("can't read after-magic")
				u.notify(nil, ErrUnexpected)
				return
			}

			if bytes.Contains(magic, lastSuffix) {
				u.notify(u.rbuf, nil)
				u.rbuf = u.rbuf[:0]
				continue
			}
			x := make([]byte, 0)
			var l int
			for {
				x = append(x, magic[:n]...)
				if offs := strings.Index(string(x), "\x0A"); offs >= 0 {

					l, err = strconv.Atoi(string(x[:offs]))
					if err != nil {
						u.Println("length", err)
						u.notify(nil, err)
						return
					}

					offs++
					if offs < len(x) {
						u.rbuf = append(u.rbuf, x[offs:]...)
						l -= len(x[offs:])
					}
					break
				}
				if n, err = u.conn.Read(magic); err != nil {
					u.Println("can't read length")
					u.notify(nil, ErrUnexpected)
				}
			}

			b := make([]byte, l)
			if n, err = u.conn.Read(b); err != nil || (n != l) {
				u.Println("issue with body")
				u.notify(nil, ErrUnexpected)
				return
			}
			u.rbuf = append(u.rbuf, b...)
		}
	}
}

type Usock struct {
	mu sync.Mutex

	conn net.Conn
	cf   context.CancelFunc

	resp chan Resp
	rbuf []byte
}

func (u *Usock) Send(b []byte, hello bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if hello {
		u.send(b)
		u.send([]byte(hello_tail))
		return
	}

	r := len(b)
	offs := 0
	for {
		if r < split {
			u.send([]byte(fmt.Sprintf("%s%d\x0A", magicPrefix, r)))
			u.send(b[offs:])
			u.send(magicPrefix)
			u.send(lastSuffix)
			return
		}
	}
}

func (u *Usock) send(b []byte) {
	if _, err := u.conn.Write(b); err != nil {
		r := Resp{Err: err}
		u.resp <- r
		u.Println("wr hello err:", err)

		u.cf()
		return
	}
}

func (u *Usock) Println(i ...interface{}) {
	fmt.Println("sock:", i)
}
