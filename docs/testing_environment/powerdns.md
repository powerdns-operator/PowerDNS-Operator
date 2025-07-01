# PowerDNS Deployment

## Deployment Options

### Option 1: Using the Official Deployment

For a full clustered PowerDNS setup, check this repository:

**[PowerDNS Deployment](https://github.com/powerdns-operator/powerdns-deployment)**

This repository provides:

- Complete PowerDNS cluster setup
- MariaDB backend configuration
- High availability configuration
- Monitoring and logging setup

### Option 2: Manual Installation (Debian/Ubuntu)

For testing or simple setups, you can install PowerDNS manually:

```bash
# Install PowerDNS on Ubuntu/Debian
apt update && apt install -y pdns-server pdns-backend-mysql

# Configure PowerDNS
# Edit /etc/powerdns/pdns.conf
```

### Option 3: Docker Deployment

```bash
# Run PowerDNS with SQLite backend
docker run -d \
  --name powerdns \
  -p 53:53 -p 53:53/udp -p 8081:8081 \
  -e PDNS_api=yes \
  -e PDNS_api_key=your-secret-key \
  -e PDNS_webserver=yes \
  -e PDNS_webserver_address=0.0.0.0 \
  -e PDNS_webserver_allow_from=0.0.0.0/0 \
  pschiffe/pdns-mysql
```

## Configuration Requirements

Ensure your PowerDNS instance has:

- **API enabled**: `api=yes` in configuration
- **Web server enabled**: `webserver=yes` for API access
- **API key configured**: Secure authentication key
- **Network access**: Accessible from Kubernetes cluster
