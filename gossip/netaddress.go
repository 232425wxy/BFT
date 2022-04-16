package gossip

import (
	protogossip "github.com/232425wxy/BFT/proto/gossip"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// EmptyNetAddress 定义一个空 NetAddress 的字符串表示
const EmptyNetAddress = "<nil-NetAddress>"

// NetAddress 定义网络上的一个对等点的信息,包括它的ID、IP地址和端口
type NetAddress struct {
	ID   ID     `json:"id"`
	IP   net.IP `json:"ip"`
	Port uint16 `json:"port"`
}

// IDAddressString 返回 id@ip:port。
// 如果主协议存在，它会从protocolHostPort中移除主协议，
// 例如会将 http://127.0.0.1:80 前面的 http:// 给去掉，只留下 127.0.0.1:80
func IDAddressString(id ID, protocolHostPort string) string {
	hostPort := removeProtocolIfDefined(protocolHostPort)
	return fmt.Sprintf("%s@%s", id, hostPort)
}

// NewNetAddress 根据提供的 TCP 地址和 ID 返回一个 NetAddress 实例。
// 如果是在测试用例，则除了 TCP 地址以外，使用其他类型的地址，都会将 IP
// 地址改为 127.0.0.1，如果是在正常运行用例过程里，传入的地址不是 TCP 类
// 型的，则会 panic，ID 不合法也会 panic。
func NewNetAddress(id ID, addr net.Addr) *NetAddress {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		if flag.Lookup("test.v") == nil {
			// 如果不是测试用例，则只支持 TCP 协议的地址
			// 所谓测试用例，就是指该函数是在 xxx_test.go 文件里运行的
			panic(fmt.Sprintf("Only TCPAddrs are supported. Got: %v", addr))
		} else {
			netAddr := NewNetAddressIPPort(net.IP("127.0.0.1"), 0)
			netAddr.ID = id
			return netAddr
		}
	}

	if err := validateID(id); err != nil {
		panic(fmt.Sprintf("Invalid ID %v: %v (addr: %v)", id, err, addr))
	}

	ip := tcpAddr.IP
	port := uint16(tcpAddr.Port)
	na := NewNetAddressIPPort(ip, port)
	na.ID = id
	return na
}

// NewNetAddressString 的入参形式如：“id@ip:port”，通过输入的参数返回一个 NetAddress 实例。
// 如果 ip 是一个 host 名，则会解析 host，得到其 ip 地址。
// 返回的错误： ErrNetAddressXxx 其中 Xxx 的取值范围包括：(NoID, Invalid, Lookup)
func NewNetAddressString(addr string) (*NetAddress, error) {
	addrWithoutProtocol := removeProtocolIfDefined(addr)
	spl := strings.Split(addrWithoutProtocol, "@")
	if len(spl) != 2 {
		return nil, ErrNetAddressNoID{addr}
	}

	// get ID
	if err := validateID(ID(spl[0])); err != nil {
		return nil, ErrNetAddressInvalid{addrWithoutProtocol, err}
	}
	var id ID
	id, addrWithoutProtocol = ID(spl[0]), spl[1]

	// get host and port
	host, portStr, err := net.SplitHostPort(addrWithoutProtocol)
	if err != nil {
		return nil, ErrNetAddressInvalid{addrWithoutProtocol, err}
	}
	if len(host) == 0 {
		return nil, ErrNetAddressInvalid{
			addrWithoutProtocol,
			errors.New("host is empty")}
	}

	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, ErrNetAddressLookup{host, err}
		}
		ip = ips[0]
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, ErrNetAddressInvalid{portStr, err}
	}

	na := NewNetAddressIPPort(ip, uint16(port))
	na.ID = id
	return na, nil
}

// NewNetAddressStrings 批量的调用 NewNetAddressString 函数，返回若干个 NetAddress 实例
func NewNetAddressStrings(addrs []string) ([]*NetAddress, []error) {
	netAddrs := make([]*NetAddress, 0)
	errs := make([]error, 0)
	for _, addr := range addrs {
		netAddr, err := NewNetAddressString(addr)
		if err != nil {
			errs = append(errs, err)
		} else {
			netAddrs = append(netAddrs, netAddr)
		}
	}
	return netAddrs, errs
}

// NewNetAddressIPPort 根据提供的 IP 和 Port 实例化一个 NetAddress
func NewNetAddressIPPort(ip net.IP, port uint16) *NetAddress {
	return &NetAddress{
		IP:   ip,
		Port: port,
	}
}

// NetAddressFromProto 将一个 Protobuf NetAddress 转换为一个原生结构体。
func NetAddressFromProto(pb protogossip.NetAddress) (*NetAddress, error) {
	ip := net.ParseIP(pb.IP)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address %v", pb.IP)
	}
	if pb.Port >= 1<<16 {
		return nil, fmt.Errorf("invalid port number %v", pb.Port)
	}
	return &NetAddress{
		ID:   ID(pb.ID),
		IP:   ip,
		Port: uint16(pb.Port),
	}, nil
}

// NetAddressesFromProto 将一个 Protobuf NetAddress 数组转换为一个本地 NetAddress 数组。
func NetAddressesFromProto(pbs []protogossip.NetAddress) ([]*NetAddress, error) {
	nas := make([]*NetAddress, 0, len(pbs))
	for _, pb := range pbs {
		na, err := NetAddressFromProto(pb)
		if err != nil {
			return nil, err
		}
		nas = append(nas, na)
	}
	return nas, nil
}

// NetAddressesToProto 将一个 NetAddress 数组转换为 protobuf NetAddress 数组
func NetAddressesToProto(nas []*NetAddress) []protogossip.NetAddress {
	pbs := make([]protogossip.NetAddress, 0, len(nas))
	for _, na := range nas {
		if na != nil {
			pbs = append(pbs, na.ToProto())
		}
	}
	return pbs
}

// ToProto 将 NetAddress 转换为 protobuf 形式的 NetAddress
func (na *NetAddress) ToProto() protogossip.NetAddress {
	return protogossip.NetAddress{
		ID:   string(na.ID),
		IP:   na.IP.String(),
		Port: uint32(na.Port),
	}
}

// Equals 比较两个 NetAddress 是否一样
func (na *NetAddress) Equals(other interface{}) bool {
	if o, ok := other.(*NetAddress); ok {
		return na.String() == o.String()
	}
	return false
}

// Same 如果两个 NetAddress 有着同样的 ID 或者 同样的拨号地址，即 ip:port，则返回 true
func (na *NetAddress) Same(other interface{}) bool {
	if o, ok := other.(*NetAddress); ok {
		if na.DialString() == o.DialString() {
			return true
		}
		if na.ID != "" && na.ID == o.ID {
			return true
		}
	}
	return false
}

// String 如果 NetAddress 含有 ID，则返回 id@ip:port，否则
// 返回 ip:port
func (na *NetAddress) String() string {
	if na == nil {
		return EmptyNetAddress
	}

	addrStr := na.DialString()
	if na.ID != "" {
		addrStr = IDAddressString(na.ID, addrStr)
	}

	return addrStr
}

// DialString 返回 ip:port
func (na *NetAddress) DialString() string {
	if na == nil {
		return "<nil-NetAddress>"
	}
	return net.JoinHostPort(
		na.IP.String(),
		strconv.FormatUint(uint64(na.Port), 10),
	)
}

// Dial 调用 net.Dial("tcp", DialString()) 给 NetAddress 拨号
func (na *NetAddress) Dial() (net.Conn, error) {
	conn, err := net.Dial("tcp", na.DialString())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// DialTimeout 调用 net.DialTimeout("tcp", DialString(), timeout) 给 NetAddress 拨号，
// 只不过加入了超时时间限制
func (na *NetAddress) DialTimeout(timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", na.DialString(), timeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Routable 判断地址是否可路由
func (na *NetAddress) Routable() bool {
	if err := na.Valid(); err != nil {
		return false
	}
	// TODO(oga) bitcoind doesn't include RFC3849 here, but should we?
	return !(na.RFC3927() || na.RFC4862() ||
		na.RFC4193() || na.RFC4843() || na.IsLocal())
}

// Valid 对于 IPv4 地址来说，如果是 “0.0.0.0” 或者 “255.255.255.255”，则返回 false，
// 对于 IPv6 地址来说，如果地址全 0，或者匹配 RFC3849 格式的地址，则返回 false
func (na *NetAddress) Valid() error {
	if err := validateID(na.ID); err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}

	if na.IP == nil {
		return errors.New("no IP")
	}
	if na.IP.IsUnspecified() || na.RFC3849() || na.IP.Equal(net.IPv4bcast) {
		return errors.New("invalid IP")
	}
	return nil
}

// HasID 判断 NetAddress 是否含有 ID
func (na *NetAddress) HasID() bool {
	return string(na.ID) != ""
}

// IsLocal 判断 NetAddress 的 IP 地址是否是环回地址
func (na *NetAddress) IsLocal() bool {
	return na.IP.IsLoopback() || zero4.Contains(na.IP)
}

// ReachabilityTo 判断两个 NetAddress 的 IP 地址是否可达
func (na *NetAddress) ReachabilityTo(o *NetAddress) int {
	const (
		Unreachable = 0
		Default     = iota
		Teredo
		Ipv6Weak
		Ipv4
		Ipv6Strong
	)
	switch {
	case !na.Routable():
		return Unreachable
	case na.RFC4380():
		switch {
		case !o.Routable():
			return Default
		case o.RFC4380():
			return Teredo
		case o.IP.To4() != nil:
			return Ipv4
		default: // ipv6
			return Ipv6Weak
		}
	case na.IP.To4() != nil:
		if o.Routable() && o.IP.To4() != nil {
			return Ipv4
		}
		return Default
	default: /* ipv6 */
		var tunnelled bool
		// Is our v6 is tunnelled?
		if o.RFC3964() || o.RFC6052() || o.RFC6145() {
			tunnelled = true
		}
		switch {
		case !o.Routable():
			return Default
		case o.RFC4380():
			return Teredo
		case o.IP.To4() != nil:
			return Ipv4
		case tunnelled:
			// only prioritise ipv6 if we aren't tunnelling it.
			return Ipv6Weak
		}
		return Ipv6Strong
	}
}

// RFC1918: IPv4 Private networks (10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12)
// RFC3849: IPv6 Documentation address  (2001:0DB8::/32)
// RFC3927: IPv4 Autoconfig (169.254.0.0/16)
// RFC3964: IPv6 6to4 (2002::/16)
// RFC4193: IPv6 unique local (FC00::/7)
// RFC4380: IPv6 Teredo tunneling (2001::/32)
// RFC4843: IPv6 ORCHID: (2001:10::/28)
// RFC4862: IPv6 Autoconfig (FE80::/64)
// RFC6052: IPv6 well known prefix (64:FF9B::/96)
// RFC6145: IPv6 IPv4 translated address ::FFFF:0:0:0/96
var rfc1918_10 = net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)}
var rfc1918_192 = net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc1918_172 = net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)}
var rfc3849 = net.IPNet{IP: net.ParseIP("2001:0DB8::"), Mask: net.CIDRMask(32, 128)}
var rfc3927 = net.IPNet{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc3964 = net.IPNet{IP: net.ParseIP("2002::"), Mask: net.CIDRMask(16, 128)}
var rfc4193 = net.IPNet{IP: net.ParseIP("FC00::"), Mask: net.CIDRMask(7, 128)}
var rfc4380 = net.IPNet{IP: net.ParseIP("2001::"), Mask: net.CIDRMask(32, 128)}
var rfc4843 = net.IPNet{IP: net.ParseIP("2001:10::"), Mask: net.CIDRMask(28, 128)}
var rfc4862 = net.IPNet{IP: net.ParseIP("FE80::"), Mask: net.CIDRMask(64, 128)}
var rfc6052 = net.IPNet{IP: net.ParseIP("64:FF9B::"), Mask: net.CIDRMask(96, 128)}
var rfc6145 = net.IPNet{IP: net.ParseIP("::FFFF:0:0:0"), Mask: net.CIDRMask(96, 128)}
var zero4 = net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(8, 32)}
var onionCatNet = ipNet("fd87:d87e:eb43::", 48, 128)

// ipNet returns a net.IPNet struct given the passed IP address string, number
// of one bits to include at the start of the mask, and the total number of bits
// for the mask.
func ipNet(ip string, ones, bits int) net.IPNet {
	return net.IPNet{IP: net.ParseIP(ip), Mask: net.CIDRMask(ones, bits)}
}

func (na *NetAddress) RFC1918() bool {
	return rfc1918_10.Contains(na.IP) ||
		rfc1918_192.Contains(na.IP) ||
		rfc1918_172.Contains(na.IP)
}
func (na *NetAddress) RFC3849() bool     { return rfc3849.Contains(na.IP) }
func (na *NetAddress) RFC3927() bool     { return rfc3927.Contains(na.IP) }
func (na *NetAddress) RFC3964() bool     { return rfc3964.Contains(na.IP) }
func (na *NetAddress) RFC4193() bool     { return rfc4193.Contains(na.IP) }
func (na *NetAddress) RFC4380() bool     { return rfc4380.Contains(na.IP) }
func (na *NetAddress) RFC4843() bool     { return rfc4843.Contains(na.IP) }
func (na *NetAddress) RFC4862() bool     { return rfc4862.Contains(na.IP) }
func (na *NetAddress) RFC6052() bool     { return rfc6052.Contains(na.IP) }
func (na *NetAddress) RFC6145() bool     { return rfc6145.Contains(na.IP) }
func (na *NetAddress) OnionCatTor() bool { return onionCatNet.Contains(na.IP) }

// removeProtocolIfDefined 移除网络地址前面的协议名，
// 例如会将 https://www.baidu.com 前面的 https:// 给去掉，
// 只留下 www.baidu.com
func removeProtocolIfDefined(addr string) string {
	if strings.Contains(addr, "://") {
		return strings.Split(addr, "://")[1]
	}
	return addr

}

func validateID(id ID) error {
	if len(id) == 0 {
		return errors.New("no ID")
	}
	idBytes, err := hex.DecodeString(string(id))
	if err != nil {
		return err
	}
	if len(idBytes) != IDByteLength {
		return fmt.Errorf("invalid hex length - got %d, expected %d", len(idBytes), IDByteLength)
	}
	return nil
}
