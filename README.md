# fast-cli
[![CI](https://img.shields.io/github/actions/workflow/status/Mr-MyDooM/fast-cli/ci.yml?branch=master&style=flat-square)](https://github.com/Mr-MyDooM/fast-cli/actions/workflows/ci.yml)
[![Software License](https://img.shields.io/badge/License-MIT-orange.svg?style=flat-square)](https://github.com/Mr-MyDooM/fast-cli/blob/master/LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/Mr-MyDooM/fast-cli)

fast-cli estimates your current internet download speed by performing a series of downloads from Netflix's fast.com servers.

Originally created by [Gus Esquivel](https://github.com/gesquive) ([gesquive/fast-cli](https://github.com/gesquive/fast-cli)). This fork modernizes the toolchain (go modules, GitHub Actions) and adds new flags (`--count`, `--bytes`, `--json`) plus latency reporting.

## Installing

### Compile
This project requires go 1.22+ to compile.

```console
git clone https://github.com/Mr-MyDooM/fast-cli.git
cd fast-cli
make build
```

Or install directly with go:

```console
go install github.com/Mr-MyDooM/fast-cli@latest
```

Optionally you can run `make install` to build and copy the executable to `/usr/local/bin/` with correct permissions.

### Download
Alternately, you can download the latest release for your platform from [github](https://github.com/Mr-MyDooM/fast-cli/releases).

Once you have an executable, make sure to copy it somewhere on your path like `/usr/local/bin` or `C:/Program Files/`.
If on a \*nix/mac system, make sure to run `chmod +x /path/to/fast-cli`.

## Usage

```console
fast-cli estimates your current internet download speed by performing a series of downloads from Netflix's fast.com servers.

Usage:
  fast-cli [flags]

Flags:
  -b, --bytes        Display speed in bytes per second instead of bits per second
  -c, --count uint   Number of parallel connections to use (default 3)
  -h, --help         help for fast-cli
      --json         Output the result as JSON
  -n, --no-https     Do not use HTTPS when connecting
  -s, --simple       Only display the result, no dynamic progress bar
      --version      Display the version number and exit
```
Optionally, a hidden debug flag is available in case you need additional output.
```console
Hidden Flags:
  -D, --debug                  Include debug statements in log output
```

## Documentation

This documentation can be found at github.com/Mr-MyDooM/fast-cli

## License

This package is made available under an MIT-style license. See LICENSE.

## Contributing

PRs are always welcome!
