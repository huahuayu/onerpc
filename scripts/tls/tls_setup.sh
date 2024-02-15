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
sudo chown -R www-data:www-data "$WEBROOT_DIR"
sudo find ${WEBROOT_DIR} -type d -exec chmod 755 {} \;
sudo find ${WEBROOT_DIR} -type f -exec chmod 644 {} \;

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

    root $WEBROOT_DIR;

    ssl_certificate $CERT_DIR/${DOMAIN}.crt;
    ssl_certificate_key $CERT_DIR/${DOMAIN}.key;

    # Explicitly handle requests to the root URL or index.html
    location = / {
        try_files /index.html =404;
    }

    location = /index.html {
        try_files $uri =404;
    }

    # Serve static files directly for specific extensions
    location ~* \.(html|css|js|png|jpg|jpeg|gif|ico)$ {
        try_files $uri =404;
    }

    # Proxy requests to onerpc service
    location / {
        # Add CORS headers
        add_header 'Access-Control-Allow-Origin' '*' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS, PUT, DELETE' always;
        add_header 'Access-Control-Allow-Headers' 'DNT,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range,Authorization' always;
        add_header 'Access-Control-Expose-Headers' 'Content-Length,Content-Range' always;

        # Handle OPTIONS request
        if (\$request_method = 'OPTIONS') {
            add_header 'Access-Control-Max-Age' 1728000;
            add_header 'Content-Type' 'text/plain charset=UTF-8';
            add_header 'Content-Length' 0;
            return 204;
        }

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
        # Add CORS headers
        add_header 'Access-Control-Allow-Origin' '*' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS, PUT, DELETE' always;
        add_header 'Access-Control-Allow-Headers' 'DNT,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range,Authorization' always;
        add_header 'Access-Control-Expose-Headers' 'Content-Length,Content-Range' always;

        # Handle OPTIONS request
        if (\$request_method = 'OPTIONS') {
            add_header 'Access-Control-Max-Age' 1728000;
            add_header 'Content-Type' 'text/plain charset=UTF-8';
            add_header 'Content-Length' 0;
            return 204;
        }

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
