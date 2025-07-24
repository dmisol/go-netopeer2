package netopeer

import (
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"testing"
)

type Bsc struct {
	Text          string `xml:",chardata"`
	Xmlns         string `xml:"xmlns,attr"`
	Configuration struct {
		Text string `xml:",chardata"`
		Mcc  int    `xml:"mcc"` // 612
		Mnc  int    `xml:"mnc"` // 04
	} `xml:"configuration"`
}

func TestResp(t *testing.T) {
	t.Skip()

	b, err := os.ReadFile(path.Join("testdata", "resp.xml"))
	if err != nil {
		t.Fatal(err)
	}
	// rpc envelop
	resp := &RpcReply{}
	if err = xml.Unmarshal(b, resp); err != nil {
		t.Fatal(err)
	}
	fmt.Println("id:", resp.MessageID)
	fmt.Println("payload:", string(resp.Data.Data))
	// content
	bsc := &Bsc{}
	if err = xml.Unmarshal(resp.Data.Data, bsc); err != nil {
		t.Fatal(err)
	}
	fmt.Println(bsc.Configuration.Mcc, bsc.Configuration.Mnc)
}
func TestReq(t *testing.T) {
	t.Skip()
	r := &Rpc{
		Xmlns:     "urn:ietf:params:xml:ns:netconf:base:1.0",
		MessageID: 12,
		Data:      `<get-data xmlns="urn:ietf:params:xml:ns:yang:ietf-netconf-nmda"><datastore xmlns:ds="urn:ietf:params:xml:ns:yang:ietf-datastores">ds:running</datastore></get-data>`,
	}
	b := r.Marshal()
	fmt.Println(b)
	fmt.Println(string(b))
}
