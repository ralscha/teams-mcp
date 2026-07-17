// Package buildinfo contains build metadata injected into release binaries.
package buildinfo

// Version is replaced by GoReleaser at link time. Development builds retain
// this value so they are never mistaken for a tagged release.
var Version = "dev"
