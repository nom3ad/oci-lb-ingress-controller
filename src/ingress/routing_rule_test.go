package ingress

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	networking "k8s.io/api/networking/v1"
)

func TestProcessImplementationSpecificPath(t *testing.T) {
	assertSuccessfulRule := func(path, expectedCondition string) {
		condition, err := processImplementationSpecificPath(path)
		if assert.NoError(t, err, "path=%q", path) {
			assert.Equal(t, expectedCondition, condition, "path=%q", path)
		}
	}
	assertInvalidRule := func(path, errorString string) {
		condition, err := processImplementationSpecificPath(path)
		if assert.Error(t, err, "path=%q", path) && assert.Equal(t, "", condition, "path=%q", path) {
			assert.Contains(t, err.Error(), errorString, "path=%q", path)
		}
	}
	assertSuccessfulRule("condition:somerawcondition", "somerawcondition")
	assertInvalidRule("condition:", "Invalid path")

	assertInvalidRule("", "Invalid path")

	// Simple path match
	assertSuccessfulRule("/example", "all(http.request.url.path eq '/example')")
	assertSuccessfulRule("/", "all(http.request.url.path eq '/')")

	// Valid wild cards
	assertSuccessfulRule("/example/*", "all(http.request.url.path sw '/example/')")
	assertSuccessfulRule("/example*", "all(http.request.url.path sw '/example')")
	assertSuccessfulRule("*/example", "all(http.request.url.path ew '/example')")
	assertSuccessfulRule("*/example/", "all(http.request.url.path ew '/example/')")
	assertSuccessfulRule("*example/", "all(http.request.url.path ew 'example/')")
	assertSuccessfulRule("*example", "all(http.request.url.path ew 'example')")
	assertSuccessfulRule("*example*", "all(http.request.url.path cw 'example')")
	assertSuccessfulRule("first*last", "all(http.request.url.path sw 'first', http.request.url.path ew 'last')")
	assertSuccessfulRule(".*", "all(http.request.url.path sw '.')")
	assertSuccessfulRule("*.*", "all(http.request.url.path cw '.')")

	// Invalid wild cards
	assertInvalidRule("fi*rst*last", "Invalid path")
	assertInvalidRule("**/example", "Invalid path")
	assertInvalidRule("example/**", "Invalid path")
	assertInvalidRule("exam*ple/*", "Invalid path")
	assertInvalidRule("*/exam*ple", "Invalid path")

	// abnormal path components
	assertSuccessfulRule("/#", "all(http.request.url.path eq '/')") // TODO: fix
	assertSuccessfulRule("ss\\", "all(http.request.url.path eq 'ss\\')")
	assertInvalidRule("/#xx", "Invalid path")

	assertInvalidRule("http://", "Invalid path")
	assertInvalidRule(":/sss", "Invalid path")
	assertInvalidRule("/%^/", "Invalid path")
}

// {"name":"S_foo_com_A87HfxlFoufIbpMVc6FEaw",
// "conditionLanguageVersion":"V1",
// "rules":

// [{"name":"jourfVC60F8aEEyz1Y8HJg","condition":"all(http.request.headers[(i 'Host')] eq (i '*.foo.com'),http.request.url.path sw '/')",
// "actions":[{"name":"FORWARD_TO_BACKENDSET","backendSetName":"wildcard-foo-com_T8080_IETss4iLd"}]
// }]}

func TestCreateHostnameCondition(t *testing.T) {
	expectations := map[string]string{
		"foo":             "http.request.headers[(i 'Host')] eq (i 'foo')",
		"www.example.com": "http.request.headers[(i 'Host')] eq (i 'www.example.com')",
		// "*.example.com":   "http.request.headers[(i 'Host')] ew (i 'example.com')",  this invalid rule.
		"*.example.com": "", // see comment at function definition
	}
	for hostname, condition := range expectations {
		assert.Equal(t, condition, createHostnameCondition(hostname), "for host: %s", hostname)
	}

}

func TestCreateRoutingRule(t *testing.T) {
	expectations := []struct {
		host           string
		path           string
		pathType       networking.PathType
		backendSetName string

		// expected
		err       string
		condition string
		action    string
	}{
		// PathTypePrefix
		{host: "www.example.com", path: "/page", pathType: networking.PathTypePrefix, backendSetName: "nginx", condition: "all(http.request.headers[(i 'Host')] eq (i 'www.example.com'),http.request.url.path sw '/page')", action: "{ BackendSetName=nginx }"},
		// PathTypeExact
		{host: "api.example.com", path: "/result/all/", pathType: networking.PathTypeExact, backendSetName: "api", condition: "all(http.request.headers[(i 'Host')] eq (i 'api.example.com'),http.request.url.path eq '/result/all/')", action: "{ BackendSetName=api }"},

		// PathTypeImplementationSpecific
		{host: "api.example.com", path: "/result/get/*", pathType: networking.PathTypeImplementationSpecific, backendSetName: "api", condition: "all(http.request.headers[(i 'Host')] eq (i 'api.example.com'),http.request.url.path sw '/result/get/')", action: "{ BackendSetName=api }"},
		{host: "api.example.com", path: "condition:http.request.cookies['cookie-name'] not eq 'cookie-value'", pathType: networking.PathTypeImplementationSpecific, backendSetName: "api", condition: "all(http.request.headers[(i 'Host')] eq (i 'api.example.com'),http.request.cookies['cookie-name'] not eq 'cookie-value')", action: "{ BackendSetName=api }"},
		{host: "api.example.com", path: "condition:all(http.request.headers[(i 'user-agent')] eq (i 'mobile'), http.request.url.query['department'] eq 'HR')", pathType: networking.PathTypeImplementationSpecific, backendSetName: "api", condition: "all(http.request.headers[(i 'Host')] eq (i 'api.example.com'),http.request.headers[(i 'user-agent')] eq (i 'mobile'), http.request.url.query['department'] eq 'HR')", action: "{ BackendSetName=api }"},
		{host: "api.example.com", path: "condition:any(http.request.url.path sw '/category', http.request.url.path ew '/id')", pathType: networking.PathTypeImplementationSpecific, backendSetName: "api", condition: "any(http.request.url.path sw '/category', http.request.url.path ew '/id')", action: "{ BackendSetName=api }"},

		// WildcardHost
		// {host: "*.example.com", path: "/page", pathType: networking.PathTypePrefix, backendSetName: "nginx", condition: "all(http.request.headers[(i 'Host')] ew (i 'example.com'),http.request.url.path sw '/page')", action: "{ BackendSetName=nginx }"},
		{host: "*.example.com", path: "/page", pathType: networking.PathTypePrefix, backendSetName: "nginx", condition: "http.request.url.path sw '/page'", action: "{ BackendSetName=nginx }"},
	}
	for i, it := range expectations {
		// assert.Equal(t, condition, createHostnameCondition(hostname), "for host: %s", hostname)
		ingresPath := networking.HTTPIngressPath{
			Path:     it.path,
			PathType: &it.pathType,
		}
		rule, err := createRoutingRule(ingresPath, it.backendSetName, it.host)
		if it.err != "" {
			assert.Error(t, err, "No error for #%d", i)
		} else {
			if assert.NoError(t, err, "Unexpected error for #%d", i) {
				assert.Equal(t, it.condition, *rule.Condition, "Rule Condition does not match for #%d", i)
				assert.Equal(t, it.action, fmt.Sprint(rule.Actions[0]), "Action does not match for #%d", i)
			}
		}
	}

}
