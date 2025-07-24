package netopeer

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func callBack(b []byte, err error) {
	fmt.Println(string(b), err)
}
func TestNP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	np, err := NewNetopeer(ctx, NetopeerSock)
	if err != nil {
		t.Fatal(err)
	}

	b := []byte(`<get-data xmlns="urn:ietf:params:xml:ns:yang:ietf-netconf-nmda"><datastore xmlns:ds="urn:ietf:params:xml:ns:yang:ietf-datastores">ds:running</datastore></get-data>`)
	for i := 0; i < 5; i++ {
		np.Request(b, callBack)
	}
	<-ctx.Done()
}
