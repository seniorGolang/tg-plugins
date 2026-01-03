package main

//go:generate env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../../dist/server.tgp .
//go:generate sh -c "shasum -a 256 ../../dist/server.tgp | cut -c 1-64 > ../../dist/server.sha256"
//go:generate sh -c "cp plugin.json ../../dist/server.json"
