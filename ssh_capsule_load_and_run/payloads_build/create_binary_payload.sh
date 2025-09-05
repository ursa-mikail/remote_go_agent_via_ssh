#chmod +x create_binary_payload.sh
#./create_binary_payload.sh

#!/bin/bash
set -e

version="3.4.2"

# Download from official site
wget https://www.openssl.org/source/openssl-$version.tar.gz

# Extract
tar -xzf openssl-$version.tar.gz

# Enter correct directory
cd openssl-$version

# Configure static build
./config no-shared
make -j$(nproc)

# Copy payload
mkdir -p ../payloads
cp apps/openssl ../payloads/openssl

# Verify payload
cd ..
file payloads/openssl
ldd payloads/openssl || echo "⚠️ Use 'otool -L' on macOS"
./payloads/openssl version -a






