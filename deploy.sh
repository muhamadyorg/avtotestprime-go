#!/bin/bash
set -e

DOMAIN="${1:-avtotestprime.uz}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"
DB_NAME="avtotestprime"
DB_USER="avtotestprime"
APP_PORT="8000"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}  AvtotestPrime Go - VPS Deploy Script${NC}"
echo -e "${GREEN}================================================${NC}"

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Xatolik: Bu skriptni root sifatida ishga tushiring!${NC}"
    echo -e "${YELLOW}Ishlatish: sudo bash deploy.sh${NC}"
    exit 1
fi

if [ -f "${PROJECT_DIR}/.env" ]; then
    echo -e "${YELLOW}Mavjud .env topildi, parollar saqlanadi${NC}"
    DB_PASS=$(grep DB_PASSWORD "${PROJECT_DIR}/.env" | head -1 | cut -d'=' -f2 | tr -d '"')
    SESSION_SECRET=$(grep SESSION_SECRET "${PROJECT_DIR}/.env" | head -1 | cut -d'=' -f2 | tr -d '"')
fi

if [ -z "$DB_PASS" ]; then
    DB_PASS=$(openssl rand -hex 24)
    echo -e "${YELLOW}Yangi DB parol yaratildi${NC}"
fi

if [ -z "$SESSION_SECRET" ]; then
    SESSION_SECRET=$(openssl rand -hex 32)
    echo -e "${YELLOW}Yangi SESSION_SECRET yaratildi${NC}"
fi

echo -e "${YELLOW}[1/7] Kerakli paketlarni o'rnatish...${NC}"
apt update -y
apt install -y git curl

if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Go o'rnatilmoqda...${NC}"
    GO_VERSION="1.24.0"
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    if ! grep -q '/usr/local/go/bin' /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    export PATH=$PATH:/usr/local/go/bin
    echo -e "${GREEN}Go $(go version) o'rnatildi!${NC}"
else
    echo -e "${GREEN}Go allaqachon o'rnatilgan: $(go version)${NC}"
fi

if ! command -v psql &> /dev/null; then
    apt install -y postgresql postgresql-contrib
    systemctl enable postgresql
    systemctl start postgresql
fi
echo -e "${GREEN}Paketlar tayyor!${NC}"

echo -e "${YELLOW}[2/7] PostgreSQL bazasini sozlash...${NC}"
systemctl start postgresql 2>/dev/null || true

PG_HBA=$(sudo -u postgres psql -tAc "SHOW hba_file;" 2>/dev/null)
if [ -n "$PG_HBA" ] && [ -f "$PG_HBA" ]; then
    if grep -q "local.*all.*all.*peer" "$PG_HBA"; then
        echo -e "${YELLOW}pg_hba.conf: peer -> md5 ga o'zgartirish...${NC}"
        sed -i 's/local\s\+all\s\+all\s\+peer/local   all             all                                     md5/' "$PG_HBA"
        systemctl reload postgresql
    fi
    if ! grep -q "host.*all.*all.*127.0.0.1.*md5\|host.*all.*all.*127.0.0.1.*scram" "$PG_HBA"; then
        echo "host    all             all             127.0.0.1/32            md5" >> "$PG_HBA"
        systemctl reload postgresql
    fi
fi

USER_EXISTS=$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" 2>/dev/null || echo "0")
if [ "$USER_EXISTS" = "1" ]; then
    sudo -u postgres psql -c "ALTER USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"
    echo -e "${GREEN}DB user parol yangilandi${NC}"
else
    sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"
    echo -e "${GREEN}DB user yaratildi${NC}"
fi

DB_EXISTS=$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" 2>/dev/null || echo "0")
if [ "$DB_EXISTS" != "1" ]; then
    sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"
    echo -e "${GREEN}Baza yaratildi!${NC}"
else
    echo -e "${GREEN}Baza allaqachon mavjud${NC}"
fi

sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"
sudo -u postgres psql -d "${DB_NAME}" -c "GRANT ALL ON SCHEMA public TO ${DB_USER};" 2>/dev/null || true

echo -e "${YELLOW}Baza ulanishini tekshirish...${NC}"
if PGPASSWORD="${DB_PASS}" psql -h 127.0.0.1 -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT 1;" > /dev/null 2>&1; then
    echo -e "${GREEN}Baza ulanishi muvaffaqiyatli!${NC}"
else
    echo -e "${RED}Baza ulanishi xato! Qo'lda tuzating.${NC}"
    exit 1
fi

echo -e "${YELLOW}[3/7] .env faylini yaratish...${NC}"
cat > "${PROJECT_DIR}/.env" <<ENVEOF
DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@127.0.0.1:5432/${DB_NAME}?sslmode=disable"
SESSION_SECRET="${SESSION_SECRET}"
PORT="${APP_PORT}"
ENVEOF
echo -e "${GREEN}.env fayli yaratildi!${NC}"

echo -e "${YELLOW}[4/7] Go dasturni kompilatsiya qilish...${NC}"
cd "$PROJECT_DIR"
export PATH=$PATH:/usr/local/go/bin
go build -buildvcs=false -o avtotestprime-server .
chmod +x avtotestprime-server
mkdir -p media/questions
echo -e "${GREEN}Dastur kompilatsiya qilindi!${NC}"

echo -e "${YELLOW}[5/7] Systemd xizmatini sozlash...${NC}"

WEB_USER="www"
if ! id -u www > /dev/null 2>&1; then
    if id -u www-data > /dev/null 2>&1; then
        WEB_USER="www-data"
    else
        WEB_USER="root"
    fi
fi
echo -e "${YELLOW}Web user: ${WEB_USER}${NC}"

cat > /etc/systemd/system/avtotestprime.service <<SERVICEEOF
[Unit]
Description=AvtotestPrime Go Server
After=network.target postgresql.service

[Service]
User=${WEB_USER}
Group=${WEB_USER}
WorkingDirectory=${PROJECT_DIR}
EnvironmentFile=${PROJECT_DIR}/.env
ExecStart=${PROJECT_DIR}/avtotestprime-server
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
SERVICEEOF

chown -R ${WEB_USER}:${WEB_USER} "$PROJECT_DIR" 2>/dev/null || chown -R root:root "$PROJECT_DIR"

systemctl daemon-reload
systemctl enable avtotestprime
systemctl restart avtotestprime

sleep 3
if systemctl is-active --quiet avtotestprime; then
    echo -e "${GREEN}Server muvaffaqiyatli ishga tushdi!${NC}"
else
    echo -e "${RED}Server xatolik. Loglar:${NC}"
    journalctl -u avtotestprime --no-pager -n 30
    exit 1
fi

echo -e "${YELLOW}[6/7] Nginx konfiguratsiya...${NC}"

NGINX_CONF=""
if [ -d "/www/server/panel/vhost/nginx" ]; then
    NGINX_CONF="/www/server/panel/vhost/nginx/${DOMAIN}.conf"
elif [ -d "/etc/nginx/sites-available" ]; then
    NGINX_CONF="/etc/nginx/sites-available/${DOMAIN}"
elif [ -d "/etc/nginx/conf.d" ]; then
    NGINX_CONF="/etc/nginx/conf.d/${DOMAIN}.conf"
fi

if [ -z "$NGINX_CONF" ]; then
    echo -e "${YELLOW}Nginx config papkasi topilmadi. Qo'lda sozlang.${NC}"
else
    if [ -f "$NGINX_CONF" ]; then
        cp "$NGINX_CONF" "${NGINX_CONF}.bak.$(date +%s)"
    fi

    LOG_DIR="/www/wwwlogs"
    if [ ! -d "$LOG_DIR" ]; then
        LOG_DIR="/var/log/nginx"
        mkdir -p "$LOG_DIR"
    fi

    cat > "$NGINX_CONF" <<NGINXEOF
server {
    listen 80;
    server_name ${DOMAIN} www.${DOMAIN};

    client_max_body_size 20M;
    access_log ${LOG_DIR}/${DOMAIN}.log;
    error_log ${LOG_DIR}/${DOMAIN}.error.log;

    location /static/ {
        alias ${PROJECT_DIR}/static/;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }

    location /media/ {
        alias ${PROJECT_DIR}/media/;
        expires 7d;
    }

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 120;
        proxy_connect_timeout 120;
    }
}
NGINXEOF

    if [ -d "/etc/nginx/sites-enabled" ] && [ ! -L "/etc/nginx/sites-enabled/${DOMAIN}" ]; then
        ln -sf "$NGINX_CONF" "/etc/nginx/sites-enabled/${DOMAIN}"
    fi

    NGINX_BIN=""
    if [ -x "/www/server/nginx/sbin/nginx" ]; then
        NGINX_BIN="/www/server/nginx/sbin/nginx"
    elif command -v nginx &> /dev/null; then
        NGINX_BIN="nginx"
    fi

    if [ -n "$NGINX_BIN" ]; then
        if $NGINX_BIN -t 2>&1; then
            $NGINX_BIN -s reload
            echo -e "${GREEN}Nginx tayyor va qayta yuklandi!${NC}"
        else
            echo -e "${RED}Nginx config xato!${NC}"
        fi
    fi
fi

echo -e "${YELLOW}[7/7] Yakuniy tekshirish...${NC}"
sleep 1
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:${APP_PORT}/ 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "302" ]; then
    echo -e "${GREEN}Sayt ishlayapti! (HTTP ${HTTP_CODE})${NC}"
else
    echo -e "${YELLOW}HTTP ${HTTP_CODE} - tekshiring: journalctl -u avtotestprime -f${NC}"
fi

echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}  Deploy yakunlandi!${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo -e "Sayt:      http://${DOMAIN}"
echo -e "Admin:     admin / admin"
echo -e "User:      user / user"
echo ""
echo -e "Yangilash: cd ${PROJECT_DIR} && git pull && sudo bash update.sh"
echo ""
echo -e "SSL:       aaPanel > Website > ${DOMAIN} > SSL > Let's Encrypt"
echo ""
echo -e "Buyruqlar:"
echo -e "  systemctl restart avtotestprime  - qayta ishga tushirish"
echo -e "  systemctl status avtotestprime   - holatni ko'rish"
echo -e "  journalctl -u avtotestprime -f   - loglarni ko'rish"
