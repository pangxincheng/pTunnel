// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// This package implements RFC5780's tests:
// - 4.3.  Determining NAT Mapping Behavior
// - 4.4.  Determining NAT Filtering Behavior
package p2p

import (
	"errors"
	"net"
	"pTunnel/utils/log"
	"time"

	"github.com/pion/stun"
)

const (
	EIM     = 0  // Endpoint-Independent Mapping
	ADM     = 1  // Address-Dependent Mapping
	APDM    = 2  // Address and Port-Dependent Mapping
	EIF     = 0  // Endpoint-Independent Filtering
	ADF     = 1  // Address-Dependent Filtering
	APDF    = 2  // Address and Port-Dependent Filtering
	UNKNOWN = 10 // Unknown
	DIRECT  = 11 // Endpoint-Independent Mapping(No NAT)
)

type stunServerConn struct {
	conn        net.PacketConn
	LocalAddr   net.Addr
	RemoteAddr  *net.UDPAddr
	OtherAddr   *net.UDPAddr
	messageChan chan *stun.Message
}

func (c *stunServerConn) Close() error {
	return c.conn.Close()
}

const (
	messageHeaderSize = 20
)

var (
	errResponseMessage = errors.New("error reading from response message channel")
	errTimedOut        = errors.New("timed out waiting for response")
	errNoOtherAddress  = errors.New("no OTHER-ADDRESS in message")
)

func CheckNATType(stunServer string, timeout int) (int, int, error) {
	// stunServer: xxx.xxx.xxx.xxx:yyyy
	mappingType, err := mappingTests(stunServer, timeout)
	if err != nil {
		return UNKNOWN, UNKNOWN, err
	}
	filteringType, err := filteringTests(stunServer, timeout)
	if err != nil {
		return mappingType, UNKNOWN, err
	}
	return mappingType, filteringType, nil
}

// RFC5780: 4.3.  Determining NAT Mapping Behavior
func mappingTests(stunServer string, timeout int) (int, error) {
	mapTestConn, err := connect(stunServer)
	defer func() {
		_ = mapTestConn.Close()
	}()
	if err != nil {
		log.Error("error connecting to STUN server: %s", err)
		return UNKNOWN, err
	}

	// Test I: Regular Binding Request
	log.Info("Mapping Test I: Regular binding request")
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	resp, err := mapTestConn.roundTrip(request, mapTestConn.RemoteAddr, timeout)
	if err != nil {
		return UNKNOWN, err
	}

	// Parse response message for XOR-MAPPED-ADDRESS and make sure OTHER-ADDRESS valid
	resps1 := parse(resp)
	if resps1.xorAddr == nil || resps1.otherAddr == nil {
		log.Info("Error: NAT discovery feature not supported by this server")
		return UNKNOWN, errNoOtherAddress
	}
	addr, err := net.ResolveUDPAddr("udp4", resps1.otherAddr.String())
	if err != nil {
		log.Info("Failed resolving OTHER-ADDRESS: %v", resps1.otherAddr)
		return UNKNOWN, err
	}
	mapTestConn.OtherAddr = addr
	log.Info("Received XOR-MAPPED-ADDRESS: %v", resps1.xorAddr)

	// Assert mapping behavior
	if resps1.xorAddr.String() == mapTestConn.LocalAddr.String() {
		log.Warn("=> NAT mapping behavior: endpoint independent (no NAT)")
		return DIRECT, nil
	}

	// Test II: Send binding request to the other address but primary port
	log.Info("Mapping Test II: Send binding request to the other address but primary port")
	oaddr := *mapTestConn.OtherAddr
	oaddr.Port = mapTestConn.RemoteAddr.Port
	resp, err = mapTestConn.roundTrip(request, &oaddr, timeout)
	if err != nil {
		return UNKNOWN, err
	}

	// Assert mapping behavior
	resps2 := parse(resp)
	log.Info("Received XOR-MAPPED-ADDRESS: %v", resps2.xorAddr)
	if resps2.xorAddr.String() == resps1.xorAddr.String() {
		log.Warn("=> NAT mapping behavior: endpoint independent")
		return EIM, nil
	}

	// Test III: Send binding request to the other address and port
	log.Info("Mapping Test III: Send binding request to the other address and port")
	resp, err = mapTestConn.roundTrip(request, mapTestConn.OtherAddr, timeout)
	if err != nil {
		return UNKNOWN, err
	}

	// Assert mapping behavior
	resps3 := parse(resp)
	log.Info("Received XOR-MAPPED-ADDRESS: %v", resps3.xorAddr)
	if resps3.xorAddr.String() == resps2.xorAddr.String() {
		log.Warn("=> NAT mapping behavior: address dependent")
		return ADM, nil
	} else {
		log.Warn("=> NAT mapping behavior: address and port dependent")
		return APDM, nil
	}
}

// RFC5780: 4.4.  Determining NAT Filtering Behavior
func filteringTests(stunServer string, timeout int) (int, error) {
	mapTestConn, err := connect(stunServer)
	defer func() {
		_ = mapTestConn.Close()
	}()
	if err != nil {
		log.Error("error connecting to STUN server: %s", err)
		return UNKNOWN, err
	}

	// Test I: Regular binding request
	log.Info("Filtering Test I: Regular binding request")
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	resp, err := mapTestConn.roundTrip(request, mapTestConn.RemoteAddr, timeout)
	if err != nil || errors.Is(err, errTimedOut) {
		return UNKNOWN, err
	}
	resps := parse(resp)
	if resps.xorAddr == nil || resps.otherAddr == nil {
		log.Warn("Error: NAT discovery feature not supported by this server")
		return UNKNOWN, errNoOtherAddress
	}
	addr, err := net.ResolveUDPAddr("udp4", resps.otherAddr.String())
	if err != nil {
		log.Info("Failed resolving OTHER-ADDRESS: %v", resps.otherAddr)
		return UNKNOWN, err
	}
	mapTestConn.OtherAddr = addr

	// Test II: Request to change both IP and port
	log.Info("Filtering Test II: Request to change both IP and port")
	request = stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	request.Add(stun.AttrChangeRequest, []byte{0x00, 0x00, 0x00, 0x06})

	resp, err = mapTestConn.roundTrip(request, mapTestConn.RemoteAddr, timeout)
	if err == nil {
		parse(resp) // just to print out the resp
		log.Warn("=> NAT filtering behavior: endpoint independent")
		return EIF, nil
	} else if !errors.Is(err, errTimedOut) {
		return UNKNOWN, nil // something else went wrong
	}

	// Test III: Request to change port only
	log.Info("Filtering Test III: Request to change port only")
	request = stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	request.Add(stun.AttrChangeRequest, []byte{0x00, 0x00, 0x00, 0x02})

	resp, err = mapTestConn.roundTrip(request, mapTestConn.RemoteAddr, timeout)
	if err == nil {
		parse(resp) // just to print out the resp
		log.Warn("=> NAT filtering behavior: address dependent")
		return ADF, nil
	} else if errors.Is(err, errTimedOut) {
		log.Warn("=> NAT filtering behavior: address and port dependent")
		return APDF, nil
	} else {
		return UNKNOWN, nil // something else went wrong
	}
}

// Parse a STUN message
func parse(msg *stun.Message) (ret struct {
	xorAddr    *stun.XORMappedAddress
	otherAddr  *stun.OtherAddress
	respOrigin *stun.ResponseOrigin
	mappedAddr *stun.MappedAddress
	software   *stun.Software
},
) {
	ret.mappedAddr = &stun.MappedAddress{}
	ret.xorAddr = &stun.XORMappedAddress{}
	ret.respOrigin = &stun.ResponseOrigin{}
	ret.otherAddr = &stun.OtherAddress{}
	ret.software = &stun.Software{}
	if ret.xorAddr.GetFrom(msg) != nil {
		ret.xorAddr = nil
	}
	if ret.otherAddr.GetFrom(msg) != nil {
		ret.otherAddr = nil
	}
	if ret.respOrigin.GetFrom(msg) != nil {
		ret.respOrigin = nil
	}
	if ret.mappedAddr.GetFrom(msg) != nil {
		ret.mappedAddr = nil
	}
	if ret.software.GetFrom(msg) != nil {
		ret.software = nil
	}
	log.Debug("%v", msg)
	log.Debug("\tMAPPED-ADDRESS:     %v", ret.mappedAddr)
	log.Debug("\tXOR-MAPPED-ADDRESS: %v", ret.xorAddr)
	log.Debug("\tRESPONSE-ORIGIN:    %v", ret.respOrigin)
	log.Debug("\tOTHER-ADDRESS:      %v", ret.otherAddr)
	log.Debug("\tSOFTWARE: %v", ret.software)
	for _, attr := range msg.Attributes {
		switch attr.Type {
		case
			stun.AttrXORMappedAddress,
			stun.AttrOtherAddress,
			stun.AttrResponseOrigin,
			stun.AttrMappedAddress,
			stun.AttrSoftware:
			break //nolint:staticcheck
		default:
			log.Debug("\t%v (l=%v)", attr, attr.Length)
		}
	}
	return ret
}

// Given an address string, returns a StunServerConn// Given an address string, returns a StunServerConn
func connect(addr string) (*stunServerConn, error) {
	log.Info("connect to STUN server: %s", addr)
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Error("error resolving UDP address: %s", err)
		return nil, err
	}
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		log.Error("error listening on UDP: %s", err)
		return nil, err
	}
	log.Info("local address: %s", c.LocalAddr())
	log.Info("remote address: %s", udpAddr.String())

	mChan := listen(c)

	return &stunServerConn{
		conn:        c,
		LocalAddr:   c.LocalAddr(),
		RemoteAddr:  udpAddr,
		messageChan: mChan,
	}, nil
}

// Send request and wait for response or timeout
func (c *stunServerConn) roundTrip(msg *stun.Message, addr net.Addr, timeout int) (*stun.Message, error) {
	_ = msg.NewTransactionID()
	log.Info("Sending to %v: (%v bytes)", addr, msg.Length+messageHeaderSize)
	log.Debug("%v", msg)
	for _, attr := range msg.Attributes {
		log.Debug("\t%v (l=%v)", attr, attr.Length)
	}
	_, err := c.conn.WriteTo(msg.Raw, addr)
	if err != nil {
		log.Warn("Error sending request to %v", addr)
		return nil, err
	}

	// Wait for response or timeout
	select {
	case m, ok := <-c.messageChan:
		if !ok {
			return nil, errResponseMessage
		}
		return m, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		log.Info("Timed out waiting for response from server %v", addr)
		return nil, errTimedOut
	}
}

// taken from https://github.com/pion/stun/blob/master/cmd/stun-traversal/main.go
func listen(conn *net.UDPConn) (messages chan *stun.Message) {
	messages = make(chan *stun.Message)
	go func() {
		for {
			buf := make([]byte, 1024)

			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				close(messages)
				return
			}
			log.Info("Response from %v: (%v bytes)", addr, n)
			buf = buf[:n]

			m := new(stun.Message)
			m.Raw = buf
			err = m.Decode()
			if err != nil {
				log.Info("Error decoding message: %v", err)
				close(messages)
				return
			}

			messages <- m
		}
	}()
	return
}
