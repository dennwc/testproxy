# Go test proxy

This package provides a way to proxy remote Go test results to the local
Go test.

Check an example of [local](./example/local) and [remote](./example/remote)
tests.

Requires Go installed on the host.

## Why?

Sometimes it's useful to compile a test binary for a different platform
using Go test on the host, and then execute it in some emulator or
isolated environment (VM/Docker).

This library provides a way to write Go test that sets up an environment,
compiles the remote test, executes it on remote and streams sub-test
results back to the host.

## License

MIT