package helper

import "net"

// GetOutboundIP - Get preferred outbound ip of this machine.
func GetOutboundIP(dnss ...string) net.IP {
	defaultAddr := net.IP("127.0.0.1")

	for _, dns := range dnss {
		conn, err := net.Dial("udp", dns)
		if err != nil {
			continue
		} else {
			defer func() { _ = conn.Close() }() // not possible leak here, only one step here (return always)

			addr, ok := conn.LocalAddr().(*net.UDPAddr)
			if !ok {
				return defaultAddr
			}

			return addr.IP
		}
	}

	return defaultAddr
}
