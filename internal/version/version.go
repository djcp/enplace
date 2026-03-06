// Package version holds the application release version.
// Override at build time with:
//
//	go build -ldflags "-X github.com/djcp/gorecipes/internal/version.Version=1.2.3"
package version

var Version = "1.0.2-alpha"
