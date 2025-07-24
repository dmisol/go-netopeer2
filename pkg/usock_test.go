package netopeer

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const (
	req1 = `<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1"><get-schema xmlns="urn:ietf:params:xml:ns:yang:ietf-netconf-monitoring"><identifier>ietf-datastores</identifier><format>yang</format></get-schema></rpc>`
	req2 = `<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="2"><get-data xmlns="urn:ietf:params:xml:ns:yang:ietf-netconf-nmda"><datastore xmlns:ds="urn:ietf:params:xml:ns:yang:ietf-datastores">ds:running</datastore></get-data></rpc>`
)

func TestUsock(t *testing.T) {
	t.Skip()

	c := make(chan Resp, 10)
	u, err := NewUsock(context.Background(), TestSock, c)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		u.Send([]byte(Hello), true)
		time.Sleep(2 * time.Second)

		u.Send([]byte(req1), false)
		u.Send([]byte(req2), false)
	}()

	x := time.NewTimer(10 * time.Second)
	for {
		select {
		case r := <-c:
			fmt.Println(string(r.Body))
			fmt.Println("_____________")
		case <-x.C:
			return
		}
	}
}
