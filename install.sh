#!/bin/bash

exe="zvm"
install_dir="$HOME/.local/bin/"

if [[ ! -f "$exe" ]]; then
    echo "Building zvm"
    go build -ldflags="-s -w" .
fi

mkdir -p "$install_dir"
cp "$exe" "$install_dir"
echo "$exe" "$install_dir"

echo "Please, ensure that $install_dir is in PATH"
