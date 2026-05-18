package ableton

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hypebeast/go-osc/osc"
)

// Client communicates with AbletonOSC over UDP.
type Client struct {
	sendAddr string
	recvConn *net.UDPConn
	handlers map[string]chan *osc.Message
	mu       sync.Mutex
}

// NewClient creates a Client that sends to sendHost:sendPort and
// receives responses on recvPort.
func NewClient(sendHost string, sendPort, recvPort int) (*Client, error) {
	recvAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", recvPort))
	if err != nil {
		return nil, fmt.Errorf("ableton: resolving recv addr: %w", err)
	}
	recvConn, err := net.ListenUDP("udp", recvAddr)
	if err != nil {
		return nil, fmt.Errorf("ableton: listening on recv port %d: %w", recvPort, err)
	}
	c := &Client{
		sendAddr: fmt.Sprintf("%s:%d", sendHost, sendPort),
		recvConn: recvConn,
		handlers: make(map[string]chan *osc.Message),
	}
	go c.readLoop()
	return c, nil
}

// Close shuts down the receive connection.
func (c *Client) Close() {
	c.recvConn.Close()
}

// Send sends an OSC message without waiting for a response.
// It sends from recvConn so that AbletonOSC replies to our receive port.
func (c *Client) Send(addr string, args ...interface{}) error {
	msg := osc.NewMessage(addr)
	for _, a := range args {
		msg.Append(a)
	}
	data, err := msg.MarshalBinary()
	if err != nil {
		return err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", c.sendAddr)
	if err != nil {
		return err
	}
	_, err = c.recvConn.WriteTo(data, udpAddr)
	return err
}

// SendRecv sends an OSC message and waits up to timeout for a response on the same address.
func (c *Client) SendRecv(addr string, timeout time.Duration, args ...interface{}) (*osc.Message, error) {
	ch := make(chan *osc.Message, 1)
	c.mu.Lock()
	c.handlers[addr] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.handlers, addr)
		c.mu.Unlock()
	}()

	if err := c.Send(addr, args...); err != nil {
		return nil, err
	}

	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("ableton: timeout waiting for response to %s", addr)
	}
}

func (c *Client) readLoop() {
	buf := make([]byte, 65535)
	for {
		n, _, err := c.recvConn.ReadFrom(buf)
		if err != nil {
			return
		}
		packet, err := osc.ParsePacket(string(buf[:n]))
		if err != nil {
			continue
		}
		msg, ok := packet.(*osc.Message)
		if !ok {
			continue
		}
		c.mu.Lock()
		ch, ok := c.handlers[msg.Address]
		c.mu.Unlock()
		if ok {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

