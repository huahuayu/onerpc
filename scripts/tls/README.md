# TLS support

The server serves the api over http, so we need to add support for https.

## Prerequisites

Install following on the server:

| Name          | Desc                                          |
|---------------|-----------------------------------------------|
| Golang        | for `go install` lego                         | 
| Nginx         | for reverse proxy                             |
| Lego          | for generating the certificate & auto-renewal |
| Apache2-utils | for cmd `htpasswd` to manage basic auth       |


Register a domain name and point it to the server.

## How to use

### Setup

Modify the env variable & run tls_setup.sh to generate the certificate.

### Teardown

Modify the env variable & run tls_teardown.sh to reset nginx to status before tls_setup.sh.

## How to add basic auth

If `htpasswd` is not installed, install it:

```bash
sudo apt-get update
sudo apt-get install apache2-utils -y
```

Create a file to store the username and password:

```bash
sudo htpasswd -c /etc/nginx/.htpasswd user1
```

You can add more users to the file:

```bash
sudo htpasswd /etc/nginx/.htpasswd anotheruser
``` 



