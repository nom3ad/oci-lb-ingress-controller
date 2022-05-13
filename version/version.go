package version

import (
	"fmt"
	"runtime"
)

var (
	// RELEASE_VERSION returns the release version
	RELEASE_VERSION = "UNKNOWN"
	// BUILD_TIMESTAMP returns time of build
	BUILD_TIMESTAMP = "UNKNOWN"
	// REPO returns the git repository URL
	REPO = "UNKNOWN"
	// COMMIT returns the short sha from git
	COMMIT = "UNKNOWN"

	OCI_SDK_VERSION = "UNKNOWN"
	K8S_SDK_VERSION = "UNKNOWN"
)

// String returns information about the release.
func String() string {
	return fmt.Sprintf(`-------------------------------------------------------------------------------
OCI Loadbalancer Ingress controller
  Version:         %v
  Build:           %v (%v/%v %v)
  BuildTimestamp:  %v
  Repository:      %v
  OCI SDK Version: %v
  K8S SDK Version: %v
-------------------------------------------------------------------------------
`, RELEASE_VERSION,
		COMMIT, runtime.GOOS, runtime.GOARCH, runtime.Version(),
		BUILD_TIMESTAMP, REPO,
		OCI_SDK_VERSION, K8S_SDK_VERSION)
}
