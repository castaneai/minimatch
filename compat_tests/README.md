# compatibility tests

This is a test to confirm compatibility between the original Open Match client and the minimatch server.

```shell
go run examples/simple1vs1/simple1vs1.go
# listening on :50504...

cd compat_tests/
go test -v ./...
```
