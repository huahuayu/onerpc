#!/bin/bash

# Variables
DOMAIN="example.com"
LEGO_PATH="$HOME/go/bin/lego"
HTPASSWD_PATH="/etc/nginx/.htpasswd"
WEBROOT_DIR="/var/www/html"

# 1. Remove Nginx configuration for domain and disable site
NGINX_CONF="/etc/nginx/sites-available/$DOMAIN"
NGINX_LINK="/etc/nginx/sites-enabled/$DOMAIN"

if [ -f "$NGINX_CONF" ]; then
    sudo rm "$NGINX_CONF"
fi

if [ -L "$NGINX_LINK" ]; then
    sudo rm "$NGINX_LINK"
fi

# 2. Delete basic auth password file for Prometheus
if [ -f "$HTPASSWD_PATH" ]; then
    echo "Removing basic auth password file for Prometheus..."
    sudo rm "$HTPASSWD_PATH"
fi

# 3. Remove cron job for SSL certificate renewal
# This searches for the cron job containing the specific command used for renewal and removes it
CRON_JOB="sudo $LEGO_PATH --email=\"$EMAIL\" --domains=\"$DOMAIN\" --http --http.webroot \"$WEBROOT_DIR\" --accept-tos --path=\"/etc/letsencrypt\" renew && sudo nginx -t && sudo systemctl reload nginx"
(crontab -l | grep -v "$CRON_JOB") | crontab -

# Reload nginx to apply changes
sudo nginx -t && sudo systemctl reload nginx

echo "Teardown completed."