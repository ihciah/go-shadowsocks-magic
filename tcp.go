package main

import (
	"github.com/ihciah/go-shadowsocks-magic/magic"
	"io"
	"net"
	"time"

	"github.com/ihciah/go-shadowsocks-magic/socks"
)

// Create a SOCKS server listening on addr and proxy to server.
func socksLocal(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("SOCKS proxy %s <-> %s", addr, server)
	tcpLocalMagic(addr, server, shadow, func(c net.Conn) (socks.Addr, error) { return socks.Handshake(c) })
}

// Create a SOCKS server listening on addr and proxy to server.
func socksLocalMagic(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("SOCKS proxy %s <-> %s", addr, server)
	getAddr := func(c net.Conn) (socks.Addr, error) {
		addr, err := socks.Handshake(c)
		return addr, err
	}
	tcpLocalMagic(addr, server, shadow, getAddr)
}

// Create a TCP tunnel from addr to target via server.
func tcpTun(addr, server, target string, shadow func(net.Conn) net.Conn) {
	tgt := socks.ParseAddr(target)
	if tgt == nil {
		logf("invalid target address %q", target)
		return
	}
	logf("TCP tunnel %s <-> %s <-> %s", addr, server, target)
	tcpLocal(addr, server, shadow, func(net.Conn) (socks.Addr, error) { return tgt, nil })
}

// Magic! Create a TCP tunnel from addr to target via server.
func tcpTunMagic(addr, server, target string, shadow func(net.Conn) net.Conn) {
	tgt := socks.ParseAddr(target)
	tgt.EnchantWithMagic(socks.AtypMagicMain)

	if tgt == nil {
		logf("invalid target address %q", target)
		return
	}
	logf("TCP tunnel %s <-> %s <-> %s", addr, server, target)
	tcpLocalMagic(addr, server, shadow, func(net.Conn) (socks.Addr, error) { return tgt, nil })
}

// Listen on addr and proxy to server to reach target from getAddr.
func tcpLocal(addr, server string, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %s", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			tgt, err := getAddr(c)
			if err != nil {

				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := []byte{}
					// block here
					for {
						_, err := c.Read(buf)
						if err, ok := err.(net.Error); ok && err.Timeout() {
							continue
						}
						logf("UDP Associate End.")
						return
					}
				}

				logf("failed to get target address: %v", err)
				return
			}

			rc, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)
			rc = shadow(rc)

			if _, err = rc.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}

			logf("proxy %s <-> %s <-> %s", c.RemoteAddr(), server, tgt)
			_, _, err = relay(rc, c)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Magic! Listen on addr and proxy to server to reach target from getAddr.
func tcpLocalMagic(addr, server string, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %s", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			tgt, err := getAddr(c)
			if err != nil {

				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := []byte{}
					// block here
					for {
						_, err := c.Read(buf)
						if err, ok := err.(net.Error); ok && err.Timeout() {
							continue
						}
						logf("UDP Associate End.")
						return
					}
				}

				logf("failed to get target address: %v", err)
				return
			}

			rc, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)
			rc = shadow(rc)

			tgt.EnchantWithMagic(socks.AtypMagicMain)
			if _, err = rc.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}

			logf("proxy with magic %s <-> %s <-> %s", c.RemoteAddr(), server, tgt)

			magic.RelayLocal(c, rc, func(dataKey [16]byte) net.Conn {
				rc, err := net.Dial("tcp", server)
				if err != nil {
					logf("failed to connect to server %v: %v", server, err)
					return nil
				}
				rc.(*net.TCPConn).SetKeepAlive(true)
				rc = shadow(rc)
				tgtChild := make([]byte, 17)
				tgtChild[0] = socks.AtypMagicChild
				copy(tgtChild[1:], dataKey[:])
				if _, err = rc.Write(tgtChild); err != nil {
					logf("failed to send target address: %v", err)
					return nil
				}
				return rc
			})

			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Listen on addr for incoming connections.
func tcpRemote(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	logf("listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			tgt, _, err := socks.ReadAddr(c)
			if err != nil {
				logf("failed to get target address: %v", err)
				return
			}

			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				logf("failed to connect to target: %v", err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)

			logf("proxy %s <-> %s", c.RemoteAddr(), tgt)
			_, _, err = relay(c, rc)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Magic! Listen on addr for incoming connections.
func tcpRemoteMagic(addr string, shadow func(net.Conn) net.Conn, GBT *magic.GlobalBufferTable) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	logf("listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			tgt, typ, err := socks.ReadAddr(c)
			if err != nil {
				logf("failed to get target address: %v", err)
				return
			}

			switch typ {
			// For normal requests
			case 0:
				rc, err := net.Dial("tcp", tgt.String())
				if err != nil {
					logf("failed to connect to target: %v", err)
					return
				}
				defer rc.Close()
				rc.(*net.TCPConn).SetKeepAlive(true)

				logf("proxy %s <-> %s", c.RemoteAddr(), tgt)
				_, _, err = relay(c, rc)
				if err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return // ignore i/o timeout
					}
					logf("relay error: %v", err)
				}
			// For requests with magic in main stream
			case socks.AtypMagicMain:
				rc, err := net.Dial("tcp", tgt.String())
				if err != nil {
					logf("failed to connect to target: %v", err)
					return
				}
				defer rc.Close()
				rc.(*net.TCPConn).SetKeepAlive(true)

				logf("proxy with magic %s <-> %s", c.RemoteAddr(), tgt)
				magic.RelayRemoteMain(c, rc, GBT, logf)
				if err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return // ignore i/o timeout
					}
					logf("magicRelayMain error: %v", err)
				}
			// For requests with magic in data streams
			case socks.AtypMagicChild:
				logf("proxy with magic (child) %s -> Key %s", c.RemoteAddr(), tgt)
				var dataKey [16]byte
				copy(dataKey[:], tgt[1:17])
				_, err = magic.RelayRemoteChild(c, dataKey, GBT, logf)
				if err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return // ignore i/o timeout
					}
					logf("magicRelayChild error: %v", err)
				}
			}
		}()
	}
}

// relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
func relay(left, right net.Conn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := io.Copy(right, left)
		right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
		ch <- res{n, err}
	}()

	n, err := io.Copy(left, right)
	right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
	left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}
