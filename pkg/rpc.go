package netopeer

import (
	"encoding/xml"
	"fmt"
)

type Rpc struct {
	Xmlns     string //`xml:"xmlns,attr"`
	MessageID int64  //`xml:"message-id,attr"`
	Data      string
}

func (r *Rpc) Marshal() []byte {
	s := fmt.Sprintf("<rpc xmlns=%q message-id=\"%d\">%s</rpc>", r.Xmlns, r.MessageID, r.Data)
	return []byte(s)
}

type RpcReply struct {
	XMLName   xml.Name `xml:"rpc-reply"`
	Text      string   `xml:",chardata"`
	Xmlns     string   `xml:"xmlns,attr"`
	MessageID int64    `xml:"message-id,attr"`
	Data      RawData  `xml:"data"`
}

type Notification struct {
	XMLName             xml.Name `xml:"notification"`
	Text                string   `xml:",chardata"`
	Xmlns               string   `xml:"xmlns,attr"`
	EventTime           string   `xml:"eventTime"` // 2025-07-25T12:00:13.76997...
	NetconfRpcExecution *RawData `xml:"netconf-rpc-execution"`
	NetconfConfigChange *RawData `xml:"netconf-config-change"`
}

type RawData struct {
	Data []byte
}

func (r *RawData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var content []byte

	for {
		token, err := d.Token()
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// Reconstruct start element
			elemStr := fmt.Sprintf("<%s", t.Name.Local)
			for _, attr := range t.Attr {
				elemStr += fmt.Sprintf(" %s=%q", attr.Name.Local, attr.Value)
			}
			elemStr += ">"
			content = append(content, []byte(elemStr)...)

		case xml.EndElement:
			if t.Name == start.Name {
				// This is our closing tag, we're done
				r.Data = content
				return nil
			}
			// Inner closing tag
			endTag := fmt.Sprintf("</%s>", t.Name.Local)
			content = append(content, []byte(endTag)...)

		case xml.CharData:
			content = append(content, []byte(t)...)

		case xml.Comment:
			content = append(content, []byte(fmt.Sprintf("<!--%s-->", string(t)))...)
		}
	}
}
