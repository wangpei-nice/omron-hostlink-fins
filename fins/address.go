package fins

import "net"

// finsAddress： A FINS device address
type finsAddress struct {
	network byte
	node    byte
	unit    byte
}

// UDPAddress: A full device address of udp
type UDPAddress struct {
	FinsAddress finsAddress
	UdpAddress  *net.UDPAddr
}

func NewUdpAddress(ip string, port int, network, node, unit byte) UDPAddress {
	return UDPAddress{
		UdpAddress: &net.UDPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		},
		FinsAddress: finsAddress{
			network: network,
			node:    node,
			unit:    unit,
		},
	}
}

// TCPAddress: A full device address of tcp
type TCPAddress struct {
	FinsAddress finsAddress
	TcpAddress  *net.TCPAddr
}

func NewTcpAddress(ip string, port int, network, node, unit byte) TCPAddress {
	return TCPAddress{
		TcpAddress: &net.TCPAddr{
			IP:   net.ParseIP(ip),
			Port: port,
		},
		FinsAddress: finsAddress{
			network: network,
			node:    node,
			unit:    unit,
		},
	}
}
