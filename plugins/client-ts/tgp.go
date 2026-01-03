package main

//go:generate env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../../dist/clientTs.tgp .
//go:generate sh -c "shasum -a 256 ../../dist/clientTs.tgp | cut -c 1-64 > ../../dist/clientTs.sha256"
//go:generate sh -c "cp plugin.json ../../dist/clientTs.json"
