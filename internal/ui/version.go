package ui

// Version is shown in the banner. Overridden at release-build time via
// -ldflags "-X gitlab.com/gto_gary/sshtui/internal/ui.Version=..."
// (see .goreleaser.yml); a plain `go build` shows this default instead.
var Version = "0.1.0"
