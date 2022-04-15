#!/usr/bin/env sh
echo "stop containers"
docker stop $(docker ps -q)

echo "delete directories"
rm -rf /root/BFT/*

echo "delete containers"
docker rm -f $(docker ps -aq)

echo "delete network bft_localnet"
docker network rm bft_localnet