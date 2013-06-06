package netwk

import (
	"bytes"
	"encoding/binary"
	"github.com/fmstephe/matching_engine/coordinator"
	"github.com/fmstephe/matching_engine/matcher"
	"github.com/fmstephe/matching_engine/msg"
	"net"
	"strconv"
	"testing"
)

// NB: There are a number of problems with these tests which are currently being ignored
// 1: Because we are communicating via UDP messages could arrive out of order, in practice they travel in-order via localhost
// 2: The messages are currently not being acked, which means that responses may be re-sent - which would confuse response checking
//    The reason this doesn't impact the tests right now is that the resend rate is so slow that the test is complete and the system
//    shut down before unacked messages are resent, this is pretty delicate

func TestRunTestSuite(t *testing.T) {
	matcher.RunTestSuite(t, newMatchTesterMaker())
}

type netwkTesterMaker struct {
	ip   [4]byte
	port int
}

func newMatchTesterMaker() matcher.MatchTesterMaker {
	return &netwkTesterMaker{ip: [4]byte{127, 0, 0, 1}, port: 1201}
}

func (m *netwkTesterMaker) Make() matcher.MatchTester {
	m.port++
	serverPort := m.port
	m.port++
	clientPort := m.port
	read := readConn(clientPort)
	write := writeConn(serverPort)
	listener, err := NewListener(strconv.Itoa(serverPort))
	if err != nil {
		panic(err)
	}
	responder := NewResponder()
	match := matcher.NewMatcher(100)
	coordinator.Coordinate(listener, responder, match, false)
	return &netwkTester{ip: m.ip, serverPort: serverPort, clientPort: clientPort, read: read, write: write}
}

type netwkTester struct {
	ip         [4]byte
	serverPort int
	clientPort int
	read       *net.UDPConn
	write      *net.UDPConn
}

func (nt *netwkTester) Send(t *testing.T, m *msg.Message) {
	nt.addNetwk(m)
	buf := bytes.NewBuffer(make([]byte, 0))
	binary.Write(buf, binary.BigEndian, m)
	nt.write.Write(buf.Bytes())
	// We always expect a server ack when sending a message
	ref := &msg.Message{}
	ref.WriteServerAckFor(m)
	nt.Expect(t, ref)
}

func (nt *netwkTester) Expect(t *testing.T, e *msg.Message) {
	nt.addNetwk(e)
	r, err := receive(nt.read)
	if err != nil {
		t.Error(err.Error())
		return
	}
	validate(t, r, e)
}

func (nt *netwkTester) Cleanup(t *testing.T) {
	m := &msg.Message{}
	m.WriteShutdown()
	nt.Send(t, m)
}

func (nt *netwkTester) addNetwk(m *msg.Message) {
	m.IP = nt.ip
	m.Port = int32(nt.clientPort)
}

func writeConn(port int) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(port))
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		panic(err)
	}
	return conn
}

func readConn(port int) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("upd", ":"+strconv.Itoa(port))
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	return conn
}

func receive(read *net.UDPConn) (*msg.Message, error) {
	s := make([]byte, msg.SizeofMessage)
	_, _, err := read.ReadFromUDP(s)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(s)
	r := &msg.Message{}
	binary.Read(buf, binary.BigEndian, r)
	return r, nil
}

func validate(t *testing.T, m, ref *msg.Message) {
	if *m != *ref {
		t.Errorf("\nExpecting: %v\nFound:     %v", ref, m)
	}
}
