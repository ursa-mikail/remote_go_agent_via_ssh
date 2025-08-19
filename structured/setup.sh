#!/usr/bin/env bash
#chmod +x setup.sh
#./setup.sh

set -e

echo "📂 Creating project structure..."

mkdir -p ssh-demo/lib

# Touch main.go
touch ssh-demo/main.go

# Touch library files
for f in upload.go remote_exec.go secret.go monitor.go automate.go log.go; do
  touch "ssh-demo/lib/$f"
done

echo "✅ Project structure created:"
tree ssh-demo
