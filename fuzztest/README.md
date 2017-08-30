This directory contains fuzz tests. It uses go-fuzz. Visit
https://github.com/dvyukov/go-fuzz and install two packages, go-fuzz-build and go-fuzz.
Then run:

```
go-fuzz-build github.com/yasushi-saito/go-netdicom/fuzztest
mkdir -p /tmp/fuzz
go-fuzz -bin fuzz-fuzz.zip -workdir /tmp/fuzz
```
