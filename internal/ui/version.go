package ui

// Version is shown in the banner. Overridden at release-build time via
// -ldflags "-X github.com/gto-gary/sshtui/internal/ui.Version=..."
// (see .goreleaser.yml); a plain `go build` shows this default instead.
var Version = "0.6.0"
