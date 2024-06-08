#!/bin/sh
set -e

current_tag() {
    local folder="$(pwd)"
    [ -n "$1" ] && folder="$1"
    git -C "$folder" describe --tags
}
VERSION=$(current_tag)
start_dir=$(pwd)

rm -rf ./release/packages
mkdir -p ./release/packages

rm -rf ./release/build
mkdir -p ./release/build

package_dir=

os="linux darwin windows"
arch="amd64 arm64"

for i in $os
do
  for j in $arch
  do
    # if i is darwin and j is 386, skip
    if [ $i = "darwin" ] && [ $j = "386" ]; then
      continue
    fi
    echo "Building $i $j"
    # build cmd/client/pTunnelClient.go
    mkdir -p ./release/build/$i-$j
    mkdir -p ./release/build/$i-$j/cert
    mkdir -p ./release/build/$i-$j/conf
    cp -r ./conf/*.example ./release/build/$i-$j/conf
    cp LICENSE ./release/build/$i-$j
    cp README.md ./release/build/$i-$j
    CGO_ENABLED=0 GOOS=$i GOARCH=$j go build -ldflags "-X pTunnel/utils/version.version=$VERSION" -o ./release/build/$i-$j/pTunnelClient cmd/client/pTunnelClient.go
    CGO_ENABLED=0 GOOS=$i GOARCH=$j go build -ldflags "-X pTunnel/utils/version.version=$VERSION" -o ./release/build/$i-$j/pTunnelServer cmd/server/pTunnelServer.go
    CGO_ENABLED=0 GOOS=$i GOARCH=$j go build -ldflags "-X pTunnel/utils/version.version=$VERSION" -o ./release/build/$i-$j/pTunnelProxy cmd/proxy/pTunnelProxy.go
    CGO_ENABLED=0 GOOS=$i GOARCH=$j go build -ldflags "-X pTunnel/utils/version.version=$VERSION" -o ./release/build/$i-$j/pTunnelGenRSAKey cmd/genRSAKey/pTunnelGenRSAKey.go
    # check whether the os is windows, if yes, add .exe suffix
    if [ $i = "windows" ]; then
      mv ./release/build/$i-$j/pTunnelClient ./release/build/$i-$j/pTunnelClient.exe
      mv ./release/build/$i-$j/pTunnelServer ./release/build/$i-$j/pTunnelServer.exe
      mv ./release/build/$i-$j/pTunnelProxy ./release/build/$i-$j/pTunnelProxy.exe
      mv ./release/build/$i-$j/pTunnelGenRSAKey ./release/build/$i-$j/pTunnelGenRSAKey.exe
    fi
    # compress the binary with zip and save it in ./release/packages
    cd ./release/build/$i-$j
    zip -r ../../packages/pTunnel-$i-$j.zip ./
    cd ../../..
  done
done
cd $start_dir
rm -rf ./release/build

echo "Build success!"
