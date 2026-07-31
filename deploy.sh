#!/bin/bash
# MikVoc Deployment Script
# Starts MikVoc server accessible from anywhere on port 8080

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="/var/log/mikvoc.log"
PID_FILE="/var/run/mikvoc.pid"

echo "=== MikVoc Deployment Script ==="

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p $PID > /dev/null 2>&1; then
        echo "✅ MikVoc is already running (PID: $PID)"
        exit 0
    else
        echo "⚠️  Stale PID file found, removing..."
        rm -f "$PID_FILE"
    fi
fi

# Create log directory if needed
sudo mkdir -p $(dirname $LOG_FILE)
sudo touch $LOG_FILE

# Kill any existing mikvoc processes
pkill -9 mikvoc 2>/dev/null || true
sleep 2

# Start new instance
cd "$SCRIPT_DIR"
echo "🚀 Starting MikVoc on all interfaces (0.0.0.0:8080)..."
nohup ./mikvoc > "$LOG_FILE" 2>&1 &
NEW_PID=$!

# Save PID
echo $NEW_PID | sudo tee $PID_FILE > /dev/null

# Wait for startup
sleep 3

# Verify it's running
if ps -p $NEW_PID > /dev/null 2>&1; then
    echo "✅ MikVoc started successfully!"
    echo "   Process ID: $NEW_PID"
    echo "   Listening on: http://0.0.0.0:8080"
    
    # Show last few log lines
    echo ""
    echo "=== Recent Logs ==="
    tail -5 "$LOG_FILE"
    
    # Get public IP
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "unknown")
    echo ""
    echo "🌍 Public Access:"
    echo "   Local:  http://localhost:8080"
    echo "   LAN:    http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080"
    echo "   Public: http://$PUBLIC_IP:8080"
    echo ""
    echo "ℹ️  To stop: sudo systemctl stop mikvoc or pkill -9 mikvoc"
    echo "ℹ️  View logs: tail -f $LOG_FILE"
else
    echo "❌ Failed to start MikVoc"
    cat "$LOG_FILE"
    exit 1
fi
