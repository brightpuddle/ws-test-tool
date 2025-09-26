# Quick Start

1. Install Go, version 1.25 or higher (on MacOS with homebrew, `brew install go`)
2. Clone this repo
3. Run `go mod tidy` to install dependencies
4. Run `go run . [classname]`, e.g. `go run . faultInst`. Alternatively, build the binary with `go build .`

* Credentials and host are optional and will be prompted if not provided.
* Subscriptions and auth token will be renewed automatically.
* The initial query results are ignored and only updated are printed

```
ACI WS monitoring tool
development build
Usage: aci-ws-tool [--apic APIC] [--usr USR] [--pwd PWD] [--http-timeout HTTP-TIMEOUT] CLASS

Positional arguments:
  CLASS                  MO Class

Options:
  --apic APIC, -a APIC   APIC host or IP
  --usr USR, -u USR      Username
  --pwd PWD, -p PWD      Password
  --http-timeout HTTP-TIMEOUT
                         HTTP timeout [default: 180]
  --help, -h             display this help and exit
  --version              display version and exit
```
