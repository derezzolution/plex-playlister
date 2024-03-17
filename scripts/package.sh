#!/bin/bash

cd $(dirname $0)

if [ ! -f "../config.json" ]; then
    echo "Could not find config.json: please create your configuration file first"
    exit 1
fi

if [ ! -f "../plex-playlister" ]; then
    echo "Could not find plex-playlister: please run build.sh first"
    exit 1
fi

if [ -f "../plex-playlister.tar.gz" ]; then
    echo "Package file plex-playlister.tar.gz has already been created"
    exit 1
fi

# Create a temp directory and clean it up afterwards regardless on whether there was an error
tempDir=$(mktemp -d)
echo "Created working temporary directory: $tempDir"
cleanup() {
    echo "Cleaning up working temporary directory: $tempDir"
    rm -rf "$tempDir"
}
trap cleanup EXIT

packageDir=$tempDir/plex-playlister
mkdir $packageDir
echo "Created package directory: $packageDir"

cp -pr ../static $packageDir/
cp -pr ../templates $packageDir/
cp -pr ../LICENSE $packageDir/
cp -pr ../config.json $packageDir/
cp -pr ../plex-playlister $packageDir/

tar cvzf "../plex-playlister.tar.gz" --directory="$packageDir" -C $tempDir .
echo "Created package file: plex-playlister.tar.gz"
