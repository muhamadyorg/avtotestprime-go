#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"

echo "AvtotestPrime yangilanmoqda..."
cd "$PROJECT_DIR"
git pull origin main || git pull origin master

export PATH=$PATH:/usr/local/go/bin
echo "Go dastur qayta kompilatsiya qilinmoqda..."
go build -buildvcs=false -o avtotestprime-server .
chmod +x avtotestprime-server

WEB_USER="www"
if ! id -u www > /dev/null 2>&1; then
    if id -u www-data > /dev/null 2>&1; then
        WEB_USER="www-data"
    else
        WEB_USER="root"
    fi
fi

chown -R ${WEB_USER}:${WEB_USER} "$PROJECT_DIR" 2>/dev/null || true
systemctl restart avtotestprime

sleep 2
echo "Tayyor! Sayt yangilandi."
echo "Holat: $(systemctl is-active avtotestprime)"
