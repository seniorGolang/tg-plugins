package main

//go:generate env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../../dist/init.tgp .
//go:generate sh -c "shasum -a 256 ../../dist/init.tgp | cut -c 1-64 > ../../dist/init.sha256"
//go:generate sh -c "cp plugin.json ../../dist/init.json"
