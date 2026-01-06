package main

//go:generate env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ../../dist/astg.tgp .
//go:generate sh -c "shasum -a 256 ../../dist/astg.tgp | cut -c 1-64 > ../../dist/astg.sha256"
//go:generate sh -c "cp plugin.json ../../dist/astg.json"

