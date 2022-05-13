package oci

import (
	"fmt"
	"strconv"

	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
)

// func GetListenerName(protocol string, port int) string {
// 	return fmt.Sprintf("%s-%d", protocol, port)
// }
func GetBackendSetName(serviceName string, protocol string, port int) string {
	// serviceName could be up to  63 chars (dns label limit)
	portStr := protocol[0:1] + strconv.Itoa(port) // 2 - 6 chars
	minPaddingLen := 6
	maxServiceNameLen := 32 - len(portStr) - minPaddingLen - 2 // considering two underscores,
	name := fmt.Sprintf("%s_%s_%s", utils.SafeSlice(serviceName, 0, maxServiceNameLen), portStr, utils.ByteAlphaNumericDigest([]byte(serviceName), 32))
	return utils.SafeSlice(name, 0, 32)
}
