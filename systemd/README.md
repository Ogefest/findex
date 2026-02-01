# Findex Systemd Configuration

## Quick Install (Recommended)

Download and install the latest release automatically:

```bash
curl -sSL https://raw.githubusercontent.com/ogefest/findex/main/install.sh | sudo bash
```

Or install a specific version:

```bash
curl -sSL https://raw.githubusercontent.com/ogefest/findex/main/install.sh | sudo bash -s v1.0.0
```

## Files

- `findex-web.service` - Web server (continuous service)
- `findex-scanner.service` - Index scanner (oneshot, triggered by timer)
- `findex-scanner.timer` - Timer for periodic scanning (daily at 3:00 AM)

## Manual Installation

### 1. Create system user

```bash
sudo useradd -r -s /sbin/nologin -d /var/lib/findex findex
```

### 2. Create directories

```bash
sudo mkdir -p /opt/findex
sudo mkdir -p /etc/findex
sudo mkdir -p /var/lib/findex
sudo chown findex:findex /var/lib/findex
```

### 3. Install binaries

```bash
# Build
go build -o findex ./cmd/findex
go build -o findex-webserver ./cmd/webserver

# Install
sudo cp findex findex-webserver /opt/findex/
sudo chmod +x /opt/findex/findex /opt/findex/findex-webserver
```

### 4. Install configuration

```bash
sudo cp index_config.yaml /etc/findex/config.yaml
sudo chown findex:findex /etc/findex/config.yaml

# Edit config - set db_path to /var/lib/findex/
sudo nano /etc/findex/config.yaml
```

Example config paths:
```yaml
indexes:
  - name: myindex
    db_path: /var/lib/findex/myindex.db
    root_paths:
      - /home
```

### 5. Install systemd units

```bash
sudo cp systemd/*.service systemd/*.timer /etc/systemd/system/
sudo systemctl daemon-reload
```

### 6. Enable and start services

```bash
# Enable web server
sudo systemctl enable findex-web.service
sudo systemctl start findex-web.service

# Enable scanner timer
sudo systemctl enable findex-scanner.timer
sudo systemctl start findex-scanner.timer

# Run initial scan manually
sudo systemctl start findex-scanner.service
```

## Management

### Check status

```bash
sudo systemctl status findex-web.service
sudo systemctl status findex-scanner.timer
sudo systemctl list-timers findex-scanner.timer
```

### View logs

```bash
sudo journalctl -u findex-web.service -f
sudo journalctl -u findex-scanner.service -f
```

### Manual scan

```bash
sudo systemctl start findex-scanner.service
```

### Restart web server

```bash
sudo systemctl restart findex-web.service
```

## Customization

### Change scan schedule

Edit `/etc/systemd/system/findex-scanner.timer`:

```ini
# Every 6 hours
OnCalendar=*-*-* 00/6:00:00

# Every hour
OnCalendar=hourly

# Weekdays at 2 AM
OnCalendar=Mon..Fri *-*-* 02:00:00
```

Then reload:
```bash
sudo systemctl daemon-reload
sudo systemctl restart findex-scanner.timer
```

### Change web server port

Edit `/etc/findex/config.yaml`:

```yaml
server:
  port: 8080
```

Or override in service file with `-listen` flag.
