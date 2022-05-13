package ingress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetListenerName(t *testing.T) {
	assert.Equal(t, "exampleDOTcom", GetListenerName("example.com"))
	assert.Equal(t, "example_DOTcom", GetListenerName("example_.com"))
	assert.Equal(t, "_exampledotDOTcom", GetListenerName("_exampledot.com"))
	assert.Equal(t, "_exampledotDOTcom", GetListenerName("_exampleDOT.com"))
	assert.Equal(t, "STARexampleDOTcom", GetListenerName("*.example.com"))
	assert.Equal(t, "STARexampleDOTcom", GetListenerName("*.example.com"))
	assert.Equal(t, "any-host", GetListenerName(""))
}

func TestGetRoutingPolicyName(t *testing.T) {
	assert.Equal(t, "example_com_Wrq9YDsieAMC3Y2DSY5R", getRoutingPolicyName("example.com"))
	assert.Equal(t, "example_com_Wrq9YDsieAMC3Y2DSY5R", getRoutingPolicyName("example.com"))
	assert.Equal(t, "S_example_com_o9KIBAUxN7GVRLfKp2", getRoutingPolicyName("*.example.com"))
	assert.Equal(t, "example_com_Yqcdq92UsJg7emuiY8cb", getRoutingPolicyName("example_.com"))
	assert.Equal(t, "X_123_example_com_gBmSB0N0toSWz4", getRoutingPolicyName("123.example_.com"))
	assert.Equal(t, "_B2M2Y8AsgTpgAmY7PhCfgiQaeLl5Abd", getRoutingPolicyName(""))
	assert.Equal(t, "a_DMF1ucDxtqgxw5niaXcmYQydAcRHa8", getRoutingPolicyName("a"))
	assert.Equal(t, 32, len(getRoutingPolicyName("")))
	assert.Equal(t, 32, len(getRoutingPolicyName("a")))
	assert.Equal(t, 32, len(getRoutingPolicyName("123.example.com")))
	assert.Equal(t, 32, len(getRoutingPolicyName("www.1234567890_1234567890_1234567890_1234567890.com")))
}
