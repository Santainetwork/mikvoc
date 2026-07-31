# MIKVOC PUBLIC ACCESS SETUP ✅

## 🚀 Server Status

**Current Configuration:**
- **Address:** `0.0.0.0:8080` (accessible from ANY network)
- **Port:** `8080`
- **Status:** ✅ Running
- **Latest Version:** v2.0.0

## 🌍 How to Access from Anywhere

### Option 1: Direct IP Access (Quick & Easy)
```bash
# Get your server's public IP
curl -s ifconfig.me

# Then access from any device/browser:
http://YOUR_PUBLIC_IP:8080
```

**Example:**
- Local Network: `http://192.168.1.100:8080`
- Public Internet: `http://203.0.113.42:8080`

### Option 2: Using Domain Name (Recommended for Production)
```bash
# 1. Point your domain A record to server IP
#    mikvoc.yourdomain.com -> YOUR_SERVER_IP

# 2. Access via HTTPS (if SSL configured):
https://mikvoc.yourdomain.com
```

### Option 3: Reverse Proxy with Nginx + SSL
```bash
# Install nginx:
sudo apt install nginx

# Create config: /etc/nginx/sites-available/mikvoc
server {
    listen 80;
    server_name mikvoc.yourdomain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Enable and restart:
sudo ln -s /etc/nginx/sites-available/mikvoc /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl restart nginx

# Setup SSL with Let's Encrypt:
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d mikvoc.yourdomain.com
```

## 🔒 Security Recommendations

### Essential (Minimum Required):
1. **Firewall Configuration**
   ```bash
   # Open only port 8080 (or 80/443 if using reverse proxy)
   sudo ufw allow 8080/tcp
   sudo ufw enable
   
   # Optional: Restrict to specific IPs only
   sudo ufw allow from YOUR_OFFICE_IP to any port 8080
   ```

2. **Set Strong Secret Key**
   ```bash
   # Generate secure secret (run once):
   openssl rand -hex 32 > .secret
   
   # Add to .env file or environment:
   export MIKVOC_SECRET=$(cat .secret)
   
   # Or add to systemd service:
   echo "MIKVOC_SECRET=your_generated_secret_here" >> /etc/default/mikvoc
   ```

3. **Basic Authentication**
   - Use `.htpasswd` with nginx or configure app-level auth

### Advanced Protection:
1. **Fail2Ban Installation**
   ```bash
   sudo apt install fail2ban
   
   # Create jail.local for MikVoc:
   cat > /etc/fail2ban/jail.local << EOF
   [mikvoc-auth]
   enabled = true
   filter = mikvoc-auth
   port = 8080
   logpath = /var/log/nginx/error.log
   maxretry = 5
   bantime = 3600
   EOF
   ```

2. **Rate Limiting** (Already built into app)
   - 10 uploads/minute per IP
   - 100 uploads/hour per IP

## 📝 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MIKVOC_PORT` | `8080` | Port to bind to |
| `MIKVOC_BIND_ADDR` | `0.0.0.0` | Bind address (all interfaces) |
| `MIKVOC_DB` | `mikvoc.db` | Database file path |
| `MIKVOC_SECRET` | *auto-generated* | Session encryption key |

### Usage Examples:

```bash
# Custom port:
MIKVOC_PORT=9000 ./mikvoc

# Custom database:
MIKVOC_DB=/data/mikvoc.db MIKVOC_PORT=8080 ./mikvoc

# With secret key:
export MIKVOC_SECRET="your-32-char-secret-key-here!"
./mikvoc
```

## 🔄 Managing the Service

### Start Server:
```bash
cd /root/.config/superpowers/worktrees/mikvoc/hotspot-visual-assets
./mikvoc
```

### Run as Background Service:
```bash
nohup ./mikvoc > /var/log/mikvoc.log 2>&1 &
echo $! > /var/run/mikvoc.pid
```

### Using systemd (Recommended):
```bash
# Create service file:
sudo nano /etc/systemd/system/mikvoc.service

# Content:
[Unit]
Description=MikVoc Hotspot Manager
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/.config/superpowers/worktrees/mikvoc/hotspot-visual-assets
ExecStart=/root/.config/superpowers/worktrees/mikvoc/hotspot-visual-assets/mikvoc
Restart=always
Environment="MIKVOC_PORT=8080"
Environment="MIKVOC_DB=/var/lib/mikvoc/db.sqlite"

[Install]
WantedBy=multi-user.target

# Enable and start:
sudo systemctl daemon-reload
sudo systemctl enable mikvoc
sudo systemctl start mikvoc

# Check status:
sudo systemctl status mikvoc

# View logs:
sudo journalctl -u mikvoc -f
```

## 🧪 Testing Accessibility

### Test from Same Machine:
```bash
curl http://localhost:8080/healthz
# Expected: {"status":"ok","db":"ok",...}
```

### Test from Another Device (LAN):
```bash
# From phone/laptop connected to same network:
curl http://SERVER_IP:8080/healthz
# Or open browser: http://SERVER_IP:8080
```

### Test from External Network:
```bash
# Using external service:
curl ifconfig.me    # Get your public IP first

# Then test from another network:
# (e.g., mobile data, different network)
http://YOUR_PUBLIC_IP:8080
```

### Port Forwarding (If Behind Router):
```bash
# If server is behind router/NAT:
# 1. Configure router to forward port 8080 to server IP
# 2. Set port forward rule: WAN 8080 → LAN_SERVER_IP:8080
# 3. Test from external network
```

## 🐛 Troubleshooting

### Server Won't Start:
```bash
# Check logs:
tail -50 /tmp/mikvoc.log

# Check port binding:
netstat -tlnp | grep 8080

# Check permissions:
ls -la mikvoc
chmod +x mikvoc

# Check database:
test -f mikvoc.db && echo "DB exists" || echo "No DB found"
```

### Can't Access from Outside:
```bash
# Check firewall:
sudo ufw status

# Check listening addresses:
netstat -tlnp | grep 8080
# Should show 0.0.0.0:8080 not 127.0.0.1:8080

# Check if process running:
pgrep -f mikvoc

# Check SELinux/AppArmor if enabled:
getenforce  # Should be Permissive or Disabled
```

### Slow Performance:
```bash
# Monitor resources:
top -p $(pgrep mikvoc)

# Check disk I/O:
iostat -x 1

# Review logs for errors:
grep -i error /tmp/mikvoc.log
```

## 📊 Monitoring

### Health Check Endpoint:
```bash
curl http://localhost:8080/healthz
# Response: {"status":"ok","db":"ok","routers_connected":X,"routers_total":Y}
```

### Real-time Logs:
```bash
# Live log tailing:
tail -f /tmp/mikvoc.log

# Or with systemd:
journalctl -u mikvoc -f
```

### Access Statistics:
```bash
# Count requests in last hour:
grep "$(date +%d/%b/%Y:%H)" /var/log/nginx/access.log | wc -l
```

## 🆘 Emergency Commands

### Stop Server:
```bash
pkill -9 mikvoc
# Or with systemd:
sudo systemctl stop mikvoc
```

### Restart Server:
```bash
# Kill and restart:
pkill -9 mikvoc
sleep 2
./mikvoc &

# Or with systemd:
sudo systemctl restart mikvoc
```

### Force Port Reuse:
```bash
# Kill all processes on port 8080:
fuser -k 8080/tcp

# Wait a moment:
sleep 3

# Then start server again:
./mikvoc
```

---

## ✨ Quick Reference Card

```bash
# START: Start server in background
nohup ./mikvoc > /var/log/mikvoc.log 2>&1 &

# STATUS: Check if running
pgrep -f mikvoc && echo "Running" || echo "Not running"

# STOP: Kill server
pkill -9 mikvoc

# LOGS: View recent logs
tail -20 /tmp/mikvoc.log

# HEALTH: Check health endpoint
curl -s http://localhost:8080/healthz | jq

# GET IP: Your server's public IP
curl -s ifconfig.me

# FIREWALL: Open port
sudo ufw allow 8080/tcp

# REBUILD: Rebuild binary
go build -o mikvoc ./cmd/mikvoc

# PUSH TO GITHUB: Push changes
git push origin main
```

---

## 🎯 Next Steps After Deployment

1. ✅ **Test Access**: Verify you can access from multiple devices/networks
2. ✅ **Backup Data**: Regularly backup `/var/lib/mikvoc/` or wherever DB is stored
3. ✅ **Monitor**: Set up logging and alerting for critical events
4. ✅ **Update**: Keep application updated with regular pulls from Git
5. ✅ **Secure**: Consider adding SSL/TLS certificate for production use

---

**Version:** v2.0.0  
**Last Updated:** Aug 1, 2026  
**Repository:** https://github.com/Santainetwork/mikvoc.git
