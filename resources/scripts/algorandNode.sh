#!/usr/bin/env bash
# Installs an indexer Algorand node on a Debian machine. Algo API listens on 0.0.0.0:8080

set -e
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit
fi
apt update
apt install -y gnupg2 curl software-properties-common unattended-upgrades
if ! command -v algod &> /dev/null; then
    curl -O https://releases.algorand.com/key.pub
    apt-key add key.pub
    apt-add-repository "deb [arch=amd64] https://releases.algorand.com/deb/ stable main"
    apt update
    apt install -y algorand-devtools
fi
algoData="/var/lib/algorand"
cd ${algoData}
rm -f genesis.json
cp -p genesis/testnet/genesis.json genesis.json
cp -p config.json.example config.json
sed -i -e 's/^.*EnableDeveloperAPI.*$/    "EnableDeveloperAPI": true,/g' config.json
sed -i -e 's/^.*IsIndexerActive.*$/    "IsIndexerActive": true,/g' config.json
sed -i -e 's/^.*EndpointAddress.*$/    "EndpointAddress": "0.0.0.0:8080",/g' config.json
sudo chown -R algorand ${algoData}
systemctl restart algorand
echo "Algod token: $(cat ${algoData}/algod.token)"