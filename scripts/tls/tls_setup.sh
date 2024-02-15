#!/bin/bash

# Variables
export DOMAIN="example.com"
export PROMETHEUS_SUBDOMAIN="prometheus.example.com"
export EMAIL="foo@example.com"
export LEGO_PATH="$HOME/go/bin/lego"
export HTPASSWD_PATH="/etc/nginx/.htpasswd"
export PROMETHEUS_USER="user1" # Change to your desired username
export PROMETHEUS_PASS="user1_pass" # Change to your desired password
export WEBROOT_DIR="/var/www/html" # Export added here

# Ensure the webroot directory exists for the HTTP challenge
sudo mkdir -p "$WEBROOT_DIR/.well-known/acme-challenge"
sudo chown -R $USER:$USER "$WEBROOT_DIR"

# Obtain SSL certificates for both main domain and subdomain
sudo "$LEGO_PATH" --email="$EMAIL" --domains="$DOMAIN" --domains="$PROMETHEUS_SUBDOMAIN" --http --http.webroot "$WEBROOT_DIR" --accept-tos --path="/etc/letsencrypt" run

export CERT_DIR="/etc/letsencrypt/certificates" # Export added here

# Create a password file for Prometheus basic authentication
echo "Creating basic auth password for Prometheus..."
# Check if `htpasswd` command exists
if ! command -v htpasswd &> /dev/null
then
    echo "htpasswd could not be found, installing apache2-utils..."
    sudo apt-get update && sudo apt-get install -y apache2-utils
fi
sudo htpasswd -cb "$HTPASSWD_PATH" "$PROMETHEUS_USER" "$PROMETHEUS_PASS"

# Nginx configuration for both main domain and subdomain
NGINX_CONF="/etc/nginx/sites-available/$DOMAIN"
NGINX_LINK="/etc/nginx/sites-enabled/$DOMAIN"

# Ensure the nginx site configuration does not already exist
if [ ! -f "$NGINX_CONF" ]; then
    sudo touch "$NGINX_CONF"
    sudo ln -s "$NGINX_CONF" "$NGINX_LINK"
fi

# Configuring Nginx as reverse proxy and setting up HTTP to HTTPS redirection
# Use envsubst to substitute environment variables within the heredoc directly
sudo envsubst '$DOMAIN $PROMETHEUS_SUBDOMAIN $WEBROOT_DIR $CERT_DIR $HTPASSWD_PATH' <<EOF | sudo tee "$NGINX_CONF" > /dev/null
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN $PROMETHEUS_SUBDOMAIN;

    location ^~ /.well-known/acme-challenge/ {
        allow all;
        root $WEBROOT_DIR;
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN;

    ssl_certificate $CERT_DIR/${DOMAIN}.crt;
    ssl_certificate_key $CERT_DIR/${DOMAIN}.key;

    # Proxy requests to onerpc service
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}

server {
    listen 443 ssl http2;
    server_name $PROMETHEUS_SUBDOMAIN;

    ssl_certificate $CERT_DIR/${DOMAIN}.crt;
    ssl_certificate_key $CERT_DIR/${DOMAIN}.key;

    location / {
        proxy_pass http://localhost:9090;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        auth_basic "Prometheus Access";
        auth_basic_user_file $HTPASSWD_PATH;
    }
}
EOF

# Ensure nginx configuration is valid and reload if so
sudo nginx -t && sudo systemctl reload nginx

# Setup automatic renewal with cron (consider the renew command's safety checks)
(crontab -l 2>/dev/null; echo "0 2 * * * sudo $LEGO_PATH --email=\"$EMAIL\" --domains=\"$DOMAIN\" --domains=\"$PROMETHEUS_SUBDOMAIN\" --http --http.webroot \"$WEBROOT_DIR\" --accept-tos --path=\"/etc/letsencrypt\" renew && sudo nginx -t && sudo systemctl reload nginx") | crontab -

# Unset all exported variables
unset DOMAIN PROMETHEUS_SUBDOMAIN EMAIL LEGO_PATH HTPASSWD_PATH PROMETHEUS_USER PROMETHEUS_PASS WEBROOT_DIR
