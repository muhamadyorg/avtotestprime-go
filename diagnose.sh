#!/bin/bash
echo "========================================"
echo "  AvtotestPrime Go - Diagnostika"
echo "  $(date)"
echo "========================================"
echo ""

PROJECT_DIR="/www/wwwroot/avtotestprime.uz"
if [ ! -d "$PROJECT_DIR" ]; then
    PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

echo "--- 1. FAYL TUZILMASI ---"
echo "PROJECT_DIR: $PROJECT_DIR"
ls -la "$PROJECT_DIR"/*.go "$PROJECT_DIR"/avtotestprime-server 2>&1
echo ""

echo "--- 2. .env FAYL ---"
cat "$PROJECT_DIR/.env" 2>&1 | sed 's/postgres:\/\/[^@]*/postgres:\/\/***:***/' | sed 's/SESSION_SECRET=.*/SESSION_SECRET=***/'
echo ""

echo "--- 3. GO VERSIYA ---"
export PATH=$PATH:/usr/local/go/bin
go version 2>&1 || echo "Go o'rnatilmagan!"
echo ""

echo "--- 4. POSTGRESQL ---"
systemctl is-active postgresql 2>&1
sudo -u postgres psql -tAc "SELECT datname FROM pg_database WHERE datname='avtotestprime';" 2>&1
sudo -u postgres psql -tAc "SELECT rolname FROM pg_roles WHERE rolname='avtotestprime';" 2>&1
echo ""

echo "--- 5. DB ULANISH ---"
DB_URL=$(grep DATABASE_URL "$PROJECT_DIR/.env" 2>/dev/null | head -1 | cut -d'"' -f2)
if [ -n "$DB_URL" ]; then
    DB_PASS=$(echo "$DB_URL" | sed 's|.*://[^:]*:\([^@]*\)@.*|\1|')
    PGPASSWORD="$DB_PASS" psql -h 127.0.0.1 -U avtotestprime -d avtotestprime -c "SELECT 1 AS db_ok;" 2>&1
else
    echo "DATABASE_URL topilmadi!"
fi
echo ""

echo "--- 6. AVTOTESTPRIME SERVICE ---"
systemctl is-active avtotestprime 2>&1
systemctl is-enabled avtotestprime 2>&1
echo ""
echo "Service fayli:"
cat /etc/systemd/system/avtotestprime.service 2>&1
echo ""
echo "Loglar (oxirgi 20 qator):"
journalctl -u avtotestprime --no-pager -n 20 2>&1
echo ""

echo "--- 7. PORT TEKSHIRUV ---"
APP_PORT=$(grep PORT "$PROJECT_DIR/.env" 2>/dev/null | grep -v DATABASE | head -1 | cut -d'=' -f2 | tr -d '"')
APP_PORT="${APP_PORT:-8000}"
echo "${APP_PORT}-portda nima ishlayapti:"
ss -tlnp | grep ":${APP_PORT}" 2>&1
echo ""
echo "Server javobi:"
curl -s -o /dev/null -w "HTTP %{http_code}" "http://127.0.0.1:${APP_PORT}/" 2>&1
echo ""
echo ""

echo "--- 8. NGINX ---"
if [ -x "/www/server/nginx/sbin/nginx" ]; then
    /www/server/nginx/sbin/nginx -t 2>&1
elif command -v nginx &> /dev/null; then
    nginx -t 2>&1
else
    echo "nginx topilmadi"
fi
echo ""
echo "Nginx config:"
CONF=""
for F in "/www/server/panel/vhost/nginx/avtotestprime.uz.conf" "/etc/nginx/sites-available/avtotestprime.uz" "/etc/nginx/conf.d/avtotestprime.uz.conf"; do
    if [ -f "$F" ]; then CONF="$F"; break; fi
done
if [ -n "$CONF" ]; then
    echo "Fayl: $CONF"
    cat "$CONF" 2>&1
else
    echo "Config topilmadi!"
fi
echo ""

echo "--- 9. WEB TEKSHIRISH ---"
echo "80-port (Nginx):"
curl -s -o /dev/null -w "HTTP %{http_code}" http://127.0.0.1:80/ 2>&1
echo ""
echo "IP orqali:"
IP=$(hostname -I | awk '{print $1}')
curl -s -o /dev/null -w "HTTP %{http_code}" "http://${IP}/" 2>&1
echo ""
echo ""

echo "========================================"
echo "  Diagnostika tugadi"
echo "========================================"
