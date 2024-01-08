// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package socks provides a SOCKS version 5 client implementation.
//
// SOCKS protocol version 5 is defined in RFC 1928.
// Username/Password authentication for SOCKS version 5 is defined in
// RFC 1929.
// taken from 	golang.org/x/net/internal/socks
package cycletls

import (

	"log"

	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"time"

)

// A Command represents a SOCKS command.
type Command int

func (cmd Command) String() string {
	switch cmd {
	case CmdConnect:
		return "socks connect"
	case cmdBind:
		return "socks bind"
	default:
		return "socks " + strconv.Itoa(int(cmd))
	}
}

// An AuthMethod represents a SOCKS authentication method.
type AuthMethod int

// A Reply represents a SOCKS command reply code.
type Reply int

func (code Reply) String() string {
	switch code {
	case StatusSucceeded:
		return "succeeded"
	case 0x01:
		return "general SOCKS server failure"
	case 0x02:
		return "connection not allowed by ruleset"
	case 0x03:
		return "network unreachable"
	case 0x04:
		return "host unreachable"
	case 0x05:
		return "connection refused"
	case 0x06:
		return "TTL expired"
	case 0x07:
		return "command not supported"
	case 0x08:
		return "address type not supported"
	default:
		return "unknown code: " + strconv.Itoa(int(code))
	}
}

// Wire protocol constants.
const (
	Version5 = 0x05

	AddrTypeIPv4 = 0x01
	AddrTypeFQDN = 0x03
	AddrTypeIPv6 = 0x04

	CmdConnect Command = 0x01 // establishes an active-open forward proxy connection
	cmdBind    Command = 0x02 // establishes a passive-open forward proxy connection

	AuthMethodNotRequired         AuthMethod = 0x00 // no authentication required
	AuthMethodUsernamePassword    AuthMethod = 0x02 // use username/password
	AuthMethodNoAcceptableMethods AuthMethod = 0xff // no acceptable authentication methods

	StatusSucceeded Reply = 0x00
)

// An Addr represents a SOCKS-specific address.
// Either Name or IP is used exclusively.
type Addr struct {
	Name string // fully-qualified domain name
	IP   net.IP
	Port int
}

type Dialer interface {
	// Dial connects to the given address via the proxy.
	Dial(network, addr string) (c net.Conn, err error)
}
// Auth contains authentication parameters that specific Dialers may require.
type Auth struct {
	User, Password string
}
func SOCKS5(network, address string, auth *Auth, forward Dialer) (Dialer, error) {
	d := NewDialer(network, address)
	if forward != nil {
		d.ProxyDial = func(_ context.Context, network string, address string) (net.Conn, error) {
			return forward.Dial(network, address)
		}
	}
	if auth != nil {
		up := UsernamePassword{
			Username: auth.User,
			Password: auth.Password,
		}
		d.AuthMethods = []AuthMethod{
			AuthMethodNotRequired,
			AuthMethodUsernamePassword,
		}
		d.Authenticate = up.Authenticate
	}
	return d, nil
}

func (a *Addr) Network() string { return "socks" }

func (a *Addr) String() string {
	if a == nil {
		return "<nil>"
	}
	port := strconv.Itoa(a.Port)
	if a.IP == nil {
		return net.JoinHostPort(a.Name, port)
	}
	return net.JoinHostPort(a.IP.String(), port)
}

// A Conn represents a forward proxy connection.
type Conn struct {
	net.Conn

	boundAddr net.Addr
}

// BoundAddr returns the address assigned by the proxy server for
// connecting to the command target address from the proxy server.
func (c *Conn) BoundAddr() net.Addr {
	if c == nil {
		return nil
	}
	return c.boundAddr
}

// A Dialer holds SOCKS-specific options.
type socks5Dialer struct {
	cmd          Command // either CmdConnect or cmdBind
	proxyNetwork string  // network between a proxy server and a client
	proxyAddress string  // proxy server address

	// ProxyDial specifies the optional dial function for
	// establishing the transport connection.
	ProxyDial func(context.Context, string, string) (net.Conn, error)

	// AuthMethods specifies the list of request authentication
	// methods.
	// If empty, SOCKS client requests only AuthMethodNotRequired.
	AuthMethods []AuthMethod

	// Authenticate specifies the optional authentication
	// function. It must be non-nil when AuthMethods is not empty.
	// It must return an error when the authentication is failed.
	Authenticate func(context.Context, io.ReadWriter, AuthMethod) error
}

// DialContext connects to the provided address on the provided
// network.
//
// The returned error value may be a net.OpError. When the Op field of
// net.OpError contains "socks", the Source field contains a proxy
// server address and the Addr field contains a command target
// address.
//
// See func Dial of the net package of standard library for a
// description of the network and address parameters.
func (d *socks5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	log.Println("aaaaadddddddddddddddddddd")
	if err := d.validateTarget(network, address); err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	if ctx == nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: errors.New("nil context")}
	}
	var err error
	var c net.Conn
	if d.ProxyDial != nil {
		c, err = d.ProxyDial(ctx, d.proxyNetwork, d.proxyAddress)
	} else {
		var dd net.Dialer
		c, err = dd.DialContext(ctx, d.proxyNetwork, d.proxyAddress)
	}
	if err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	a, err := d.connect(ctx, c, address)
	if err != nil {
		c.Close()
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	return &Conn{Conn: c, boundAddr: a}, nil
}

// DialWithConn initiates a connection from SOCKS server to the target
// network and address using the connection c that is already
// connected to the SOCKS server.
//
// It returns the connection's local address assigned by the SOCKS
// server.
func (d *socks5Dialer) DialWithConn(ctx context.Context, c net.Conn, network, address string) (net.Addr, error) {
	if err := d.validateTarget(network, address); err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	if ctx == nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: errors.New("nil context")}
	}
	a, err := d.connect(ctx, c, address)
	if err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	return a, nil
}

// Dial connects to the provided address on the provided network.
//
// Unlike DialContext, it returns a raw transport connection instead
// of a forward proxy connection.
//
// Deprecated: Use DialContext or DialWithConn instead.
func (d *socks5Dialer) Dial(network, address string) (net.Conn, error) {
	if err := d.validateTarget(network, address); err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	var err error
	var c net.Conn
	if d.ProxyDial != nil {
		c, err = d.ProxyDial(context.Background(), d.proxyNetwork, d.proxyAddress)
	} else {
		c, err = net.Dial(d.proxyNetwork, d.proxyAddress)
	}
	if err != nil {
		proxy, dst, _ := d.pathAddrs(address)
		return nil, &net.OpError{Op: d.cmd.String(), Net: network, Source: proxy, Addr: dst, Err: err}
	}
	if _, err := d.DialWithConn(context.Background(), c, network, address); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (d *socks5Dialer) validateTarget(network, address string) error {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return errors.New("network not implemented")
	}
	switch d.cmd {
	case CmdConnect, cmdBind:
	default:
		return errors.New("command not implemented")
	}
	return nil
}
func splitHostPort(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	portnum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	if 1 > portnum || portnum > 0xffff {
		return "", 0, errors.New("port number out of range " + port)
	}
	return host, portnum, nil
}

func (d *socks5Dialer) pathAddrs(address string) (proxy, dst net.Addr, err error) {
	for i, s := range []string{d.proxyAddress, address} {
		host, port, err := splitHostPort(s)
		if err != nil {
			return nil, nil, err
		}
		a := &Addr{Port: port}
		a.IP = net.ParseIP(host)
		if a.IP == nil {
			a.Name = host
		}
		if i == 0 {
			proxy = a
		} else {
			dst = a
		}
	}
	return
}

// NewDialer returns a new Dialer that dials through the provided
// proxy server's network and address.
func NewDialer(network, address string) *socks5Dialer {
	return &socks5Dialer{proxyNetwork: network, proxyAddress: address, cmd: CmdConnect}
}

const (
	authUsernamePasswordVersion = 0x01
	authStatusSucceeded         = 0x00
)

// UsernamePassword are the credentials for the username/password
// authentication method.
type UsernamePassword struct {
	Username string
	Password string
}

// Authenticate authenticates a pair of username and password with the
// proxy server.
func (up *UsernamePassword) Authenticate(ctx context.Context, rw io.ReadWriter, auth AuthMethod) error {
	switch auth {
	case AuthMethodNotRequired:
		return nil
	case AuthMethodUsernamePassword:
		if len(up.Username) == 0 || len(up.Username) > 255 || len(up.Password) > 255 {
			return errors.New("invalid username/password")
		}
		b := []byte{authUsernamePasswordVersion}
		b = append(b, byte(len(up.Username)))
		b = append(b, up.Username...)
		b = append(b, byte(len(up.Password)))
		b = append(b, up.Password...)
		// TODO(mikio): handle IO deadlines and cancelation if
		// necessary
		if _, err := rw.Write(b); err != nil {
			return err
		}
		if _, err := io.ReadFull(rw, b[:2]); err != nil {
			return err
		}
		if b[0] != authUsernamePasswordVersion {
			return errors.New("invalid username/password version")
		}
		if b[1] != authStatusSucceeded {
			return errors.New("username/password authentication failed")
		}
		return nil
	}
	return errors.New("unsupported authentication method " + strconv.Itoa(int(auth)))
}
var (
	noDeadline   = time.Time{}
	aLongTimeAgo = time.Unix(1, 0)
)


func (d *socks5Dialer) connect(ctx context.Context, c net.Conn, address string) (_ net.Addr, ctxErr error) {
	log.Println("aaaaaaaaa")
	host, port, err := splitHostPort(address)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok && !deadline.IsZero() {
		c.SetDeadline(deadline)
		defer c.SetDeadline(noDeadline)
	}
	if ctx != context.Background() {
		errCh := make(chan error, 1)
		done := make(chan struct{})
		defer func() {
			close(done)
			if ctxErr == nil {
				ctxErr = <-errCh
			}
		}()
		go func() {
			select {
			case <-ctx.Done():
				c.SetDeadline(aLongTimeAgo)
				errCh <- ctx.Err()
			case <-done:
				errCh <- nil
			}
		}()
	}

	b := make([]byte, 0, 6+len(host)) // the size here is just an estimate
	b = append(b, Version5)
	if len(d.AuthMethods) == 0 || d.Authenticate == nil {
		b = append(b, 1, byte(AuthMethodNotRequired))
	} else {
		ams := d.AuthMethods
		if len(ams) > 255 {
			return nil, errors.New("too many authentication methods")
		}
		b = append(b, byte(len(ams)))
		for _, am := range ams {
			b = append(b, byte(am))
		}
	}
	if _, ctxErr = c.Write(b); ctxErr != nil {
		return
	}

	if _, ctxErr = io.ReadFull(c, b[:2]); ctxErr != nil {
		return
	}
	if b[0] != Version5 {
		return nil, errors.New("unexpected protocol version " + strconv.Itoa(int(b[0])))
	}
	am := AuthMethod(b[1])
	if am == AuthMethodNoAcceptableMethods {
		return nil, errors.New("no acceptable authentication methods")
	}
	if d.Authenticate != nil {
		if ctxErr = d.Authenticate(ctx, c, am); ctxErr != nil {
			return
		}
	}

	b = b[:0]
	b = append(b, Version5, byte(d.cmd), 0)
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			b = append(b, AddrTypeIPv4)
			b = append(b, ip4...)
		} else if ip6 := ip.To16(); ip6 != nil {
			b = append(b, AddrTypeIPv6)
			b = append(b, ip6...)
		} else {
			return nil, errors.New("unknown address type")
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("FQDN too long")
		}
		b = append(b, AddrTypeFQDN)
		b = append(b, byte(len(host)))
		b = append(b, host...)
	}
	b = append(b, byte(port>>8), byte(port))
	if _, ctxErr = c.Write(b); ctxErr != nil {
		return
	}

	if _, ctxErr = io.ReadFull(c, b[:4]); ctxErr != nil {
		return
	}
	if b[0] != Version5 {
		return nil, errors.New("unexpected protocol version " + strconv.Itoa(int(b[0])))
	}
	if cmdErr := Reply(b[1]); cmdErr != StatusSucceeded {
		return nil, errors.New("unknown error " + cmdErr.String())
	}
	if b[2] != 0 {
		return nil, errors.New("non-zero reserved field")
	}
	l := 2
	var a Addr
	switch b[3] {
	case AddrTypeIPv4:
		l += net.IPv4len
		a.IP = make(net.IP, net.IPv4len)
	case AddrTypeIPv6:
		l += net.IPv6len
		a.IP = make(net.IP, net.IPv6len)
	case AddrTypeFQDN:
		if _, err := io.ReadFull(c, b[:1]); err != nil {
			return nil, err
		}
		l += int(b[0])
	default:
		return nil, errors.New("unknown address type " + strconv.Itoa(int(b[3])))
	}
	if cap(b) < l {
		b = make([]byte, l)
	} else {
		b = b[:l]
	}
	if _, ctxErr = io.ReadFull(c, b); ctxErr != nil {
		return
	}
	if a.IP != nil {
		copy(a.IP, b)
	} else {
		a.Name = string(b[:len(b)-2])
	}
	a.Port = int(b[len(b)-2])<<8 | int(b[len(b)-1])
	return &a, nil
}
