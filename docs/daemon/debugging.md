# Daemon Debugging & Troubleshooting

This guide provides diagnostic workflows and resolutions for common daemon issues.

## 1. Quick Health Check

Check daemon status via CLI:
```bash
daneel daemon status
```

Verify the Unix socket manually using `nc` (netcat):
```bash
echo '{"method":"ping"}' | nc -U ~/.local/state/daneel/daneel.sock
# Expected response: {"result":"pong"}
```

## 2. Common Issues & Resolutions

### Stale Socket or PID File
If the machine crashed or the process was forcefully killed (`SIGKILL`):
```bash
# Check if a process is still holding the PID
cat ~/.local/state/daneel/daneel.pid
ps -p $(cat ~/.local/state/daneel/daneel.pid 2>/dev/null)

# Clean up stale files if no process is running
rm -f ~/.local/state/daneel/daneel.sock ~/.local/state/daneel/daneel.pid
```

### Systemd Service Diagnostics
If running via `systemd --user`:
```bash
# View live service logs
journalctl --user -u daneel -f

# Check detailed status and restart history
systemctl --user status daneel

# Restart service manually
systemctl --user restart daneel
```

### Permission Denied on Socket Directory
The daemon expects directory permissions `0700` for user security:
```bash
ls -ld ~/.local/state/daneel
chmod 700 ~/.local/state/daneel
```

## 3. Interactive Protocol Testing

You can send raw JSON-RPC requests over the socket to test functionality:

```bash
# Query active context
echo '{"method":"get_active_context"}' | nc -U ~/.local/state/daneel/daneel.sock

# Send a test notification
echo '{"method":"publish_ui_event","params":{"action":3,"target_path":"","payload_json":"{\"title\":\"Test\",\"message\":\"Hello from socket\"}"}}' | nc -U ~/.local/state/daneel/daneel.sock
```
