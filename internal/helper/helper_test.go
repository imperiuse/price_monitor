package helper

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetOutboundIP(t *testing.T) {
	assert.Equal(t, net.IP{0x31, 0x32, 0x37, 0x2e, 0x30, 0x2e, 0x30, 0x2e, 0x31}, GetOutboundIP())
	assert.NotNil(t, GetOutboundIP("8.8.8.8", "1.1.1.1", "127.0.0.1"))
}
