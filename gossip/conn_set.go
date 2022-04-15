package gossip

import (
	"net"
	"sync"
)

type connSetItem struct {
	conn net.Conn
	ips  []net.IP
}

type ConnSet struct {
	sync.RWMutex

	conns map[string]connSetItem
}

// NewConnSet returns a ConnSet implementation.
func NewConnSet() *ConnSet {
	return &ConnSet{
		conns: map[string]connSetItem{},
	}
}

func (cs *ConnSet) HasConn(c net.Conn) bool {
	cs.RLock()
	defer cs.RUnlock()

	_, ok := cs.conns[c.RemoteAddr().String()]

	return ok
}

func (cs *ConnSet) HasIP(ip net.IP) bool {
	cs.RLock()
	defer cs.RUnlock()

	for _, c := range cs.conns {
		for _, known := range c.ips {
			if known.Equal(ip) {
				return true
			}
		}
	}

	return false
}

func (cs *ConnSet) RemoveConn(c net.Conn) {
	cs.Lock()
	defer cs.Unlock()

	delete(cs.conns, c.RemoteAddr().String())
}

func (cs *ConnSet) RemoveAddr(addr net.Addr) {
	cs.Lock()
	defer cs.Unlock()

	delete(cs.conns, addr.String())
}

func (cs *ConnSet) Set(c net.Conn, ips []net.IP) {
	cs.Lock()
	defer cs.Unlock()

	cs.conns[c.RemoteAddr().String()] = connSetItem{
		conn: c,
		ips:  ips,
	}
}
