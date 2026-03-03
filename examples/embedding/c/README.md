# C embedding example

This example shows how to consume the `c-shared` bindings emitted by `cmd/gocdm-bindings`.

## Build shared library

```bash
go build -buildmode=c-shared -o libgocdm.so ./cmd/gocdm-bindings
```

## Build example

```bash
cc -o example main.c -L. -lgocdm -Wl,-rpath,'$ORIGIN'
```

## Run

```bash
./example
```
