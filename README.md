## Run

1. Print run command:
  ```bash
  echo "PORT=80 GITHUB_TOKEN='${GITHUB_TOKEN}' GITHUB_WEBHOOK_SECRET='${GITHUB_WEBHOOK_SECRET}' TELEGRAM_TOKEN='${TELEGRAM_TOKEN}' go run main.go"
  ```
2. `cd ~/go/src/github.com/andreyst/tracker-messenger-bridge`
3. run with printed command

## Sync

```bash
while true; do rsync -az . root@198.199.80.196:/root/go/src/github.com/andreyst/tracker-messenger-bridge 2>/dev/null; sleep 1; done
```

## Build
1. `$ brew install sqlite3`
2. 