# Contributing

## Building

```sh
make build
```

Builds the `sshtui` binary into `bin/`. `make install` installs it via
`go install ./cmd/sshtui` instead.

## Testing

```sh
make test
```

Runs the full test suite (`go test ./...`), including the config-editing
golden tests under `internal/config/testdata/`, and the UI/actions/history/
reachability/dispatch/shell-init suites under `internal/*/*_test.go` and
`cmd/sshtui/*_test.go`.

## Vetting

```sh
make vet
```

Runs `go vet ./...`. Run this along with `gofmt -l .` before committing.
