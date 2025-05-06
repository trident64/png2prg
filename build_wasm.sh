#!/bin/bash
set -ex  # Enable debugging and exit on error

echo "Setting up web directory..."
mkdir -p web

echo "Building WebAssembly binary..."
export GOOS=js
export GOARCH=wasm
go build -v -o web/png2prg.wasm ./cmd/wasm  # Updated path here

# Verify the file was created
if [ ! -f "web/png2prg.wasm" ]; then
    echo "ERROR: Failed to build png2prg.wasm"
    exit 1
fi

echo "Copying wasm_exec.js..."
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" web/

echo "Copying index.html to web directory..."
if [ -f "/workspaces/trident-c64-png2prg/index.html" ]; then
    cp /workspaces/trident-c64-png2prg/index.html web/
fi

echo "Checking web directory contents:"
ls -la web/

echo "Build complete! Files are in the web directory."