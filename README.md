# linkedbot - LinkedIn Automation CLI (PoC)

Proof-of-concept Go CLI that automates LinkedIn using the Rod browser automation library. It logs in, searches for targets, sends connection requests with a personalized note, tracks state in SQLite, detects acceptances, and sends follow-up messages. Includes stealth techniques to mimic human behavior.

WARNING: For educational purposes only. Automating LinkedIn may violate their Terms of Service. Use responsibly and at your own risk.

## Architecture

```
cmd/linkedbot/main.go        - CLI entry; subcommands
internal/config              - Config load/validation (YAML + env)
internal/logging             - Structured logging wrapper (slog)
internal/browser             - Rod launcher, user agent, viewport, helpers
internal/auth                - Login, cookie persistence, session validation
internal/stealth             - Human-like movements, timing, typing, scroll
internal/search              - Search people, scrape profile cards, pagination
internal/connection          - Send connection requests with template note
internal/messaging           - Detect acceptances & send follow-ups
internal/store               - SQLite persistence & queries
internal/models              - Data models
```

ASCII Diagram:

```
+-------------+     +-----------+     +-----------+     +-----------+
|   CLI       | --> |  Auth     | --> |  Search   | --> | Connection|
+-------------+     +-----------+     +-----------+     +-----------+
       |                   |                 |                 |
       v                   v                 v                 v
   Config & Log        Browser/Rod       Stealth utils      Store/DB
       |                                                       |
       +--------------------------- Messaging -----------------+
```

## Prerequisites

- Go 1.22+
- Chrome/Chromium installed (Rod will try to locate it)
- SQLite (via modernc.org/sqlite driver)

## Quick Start

### Windows (PowerShell)

```powershell
# 1. Install dependencies
go mod tidy

# 2. Build
go build -o linkedbot.exe .\cmd\linkedbot

# 3. Setup configuration
Copy-Item .env.example .env
Copy-Item config.example.yaml config.yaml

# 4. Edit .env and add your credentials
notepad .env

# 5. Test setup
.\test-setup.ps1

# 6. Login
.\linkedbot.exe login

# 7. Try a search
.\linkedbot.exe search --title "Software Engineer" --location "India" --limit 10
```

### Linux/Mac (Bash)

```bash
# 1. Install dependencies
go mod tidy

# 2. Build
go build -o linkedbot ./cmd/linkedbot

# 3. Setup configuration
cp .env.example .env
cp config.example.yaml config.yaml

# 4. Edit .env and add your credentials
nano .env

# 5. Test setup
chmod +x test-setup.sh
./test-setup.sh

# 6. Login
./linkedbot login

# 7. Try a search
./linkedbot search --title "Software Engineer" --location "India" --limit 10
```

## Configuration

### Required Environment Variables (.env file)

- `LINKEDIN_EMAIL` - Your LinkedIn account email
- `LINKEDIN_PASSWORD` - Your LinkedIn account password

### Optional Environment Variables

- `LINKEDBOT_DB_PATH` - Database file path (default: linkedbot.db)
- `LINKEDBOT_LOG_LEVEL` - Logging level: debug|info|warn|error (default: info)
- `LINKEDBOT_HEADLESS` - Run browser in headless mode: true|false (default: false)

### Configuration File (config.yaml)

The config.yaml file contains:
- Search defaults (title, company, location, keywords)
- Daily limits for connections and messages
- Stealth settings (delays, viewport, active hours)
- Message templates for connections and follow-ups

Edit these values to customize behavior.

## Usage

```bash
# ensure login/cookies
./linkedbot --config config.yaml login

# search for targets
./linkedbot search --title "Software Engineer" --location "India" --keywords "golang" --limit 100

# send connections (respects daily limit)
./linkedbot send-connections --limit 20

# send follow-up messages
./linkedbot send-messages --limit 50

# run a composed flow (controlled by env RUN_* flags)
./linkedbot run-all
```

## Notes on Selectors

LinkedIn UI changes frequently. The provided CSS selectors are reasonable but may require adjustment. Check the DOM and tweak selectors in:
- internal/search/search.go
- internal/connection/connection.go
- internal/messaging/messaging.go

## Stealth Techniques Implemented

- Human-like mouse movement via bezier paths with jitter
- Randomized delays and think time
- User agent randomization and viewport randomization
- navigator.webdriver masking
- Random scrolling and small reverse scrolls
- Realistic typing with typos and corrections
- Hover/wander on elements (basic)
- Active hours schedule checks
- Rate limiting by daily caps and random delays between actions

## Persistence

SQLite database is created automatically (linkedbot.db by default). Tables:
- profiles
- message_logs
- run_logs (optional)

Idempotency: Upsert on profile URL; message logs are append-only.

## Legal/Ethical

- This is a PoC. Do not abuse it.
- May violate site policies. Use with caution, for research purposes only.

## Troubleshooting

### Login Issues

1. **Login fails with "checkpoint/verification"**
   - LinkedIn may be asking for verification
   - Solution: Login manually to LinkedIn in a normal browser from the same machine first
   - Then try again - LinkedIn will recognize your device

2. **"Invalid credentials" error**
   - Double-check your `.env` file has correct `LINKEDIN_EMAIL` and `LINKEDIN_PASSWORD`
   - Ensure no extra spaces or quotes around values
   - Make sure `.env` file is in the same directory as the executable

3. **Login loops**
   - Delete `.cache/cookies.json` and try fresh login: `del .cache\cookies.json` (Windows) or `rm -rf .cache` (Unix)
   - Check if antivirus is blocking the browser automation

### Search Issues

1. **"No links found on first page"**
   - LinkedIn may have changed their HTML structure again
   - Check `search_fail_full.html` and `search_fail.png` files created in the directory
   - The selectors in `internal/search/search.go` may need updating

2. **Search returns 0 results**
   - Try broadening your search criteria (remove company/location filters)
   - LinkedIn search might be rate-limited - wait 30 mins and try again
   - Ensure you're logged in successfully first: `.\linkedbot.exe login`

3. **Getting rate limited**
   - Increase `min_delay_ms` and `max_delay_ms` in `config.yaml`
   - Reduce `max_profiles_per_search` limit
   - Run searches during off-peak hours

### Connection Issues

1. **Connect button not found**
   - Screenshot saved as `connect_button_fail.png` - check what's on the page
   - Some profiles don't allow connection requests (e.g., LinkedIn influencers with "Follow" only)
   - Profile might already be connected

2. **Connection requests not sending**
   - Check daily limit hasn't been reached: `max_connections_per_day` in config.yaml
   - LinkedIn may have temporary restrictions on your account
   - Wait 24 hours and try again

### Message Issues

1. **Message button not found**
   - Connection might not be accepted yet
   - Run acceptance detection first: the app checks this automatically
   - Some connections don't allow messaging

2. **Messages not sending**
   - Check daily limit: `max_messages_per_day` in config.yaml
   - LinkedIn messaging might be restricted on your account

### General Issues

1. **Chrome/Browser not found**
   - Install Google Chrome or Chromium browser
   - Rod will auto-download if needed, but manual Chrome install is more reliable

2. **Database locked errors**
   - Close any other instances of the application
   - Delete `linkedbot.db` to start fresh (you'll lose history)

3. **Slow performance**
   - Increase timeout values in code if needed
   - Close other Chrome instances
   - Check your internet connection speed

4. **Antivirus blocking**
   - Windows Defender might flag browser automation
   - Add exception for the linkedbot.exe
   - Or build from source with: `go build -o linkedbot.exe .\cmd\linkedbot`

### Debug Mode

For detailed logging, set environment variable:
```powershell
$env:LINKEDBOT_LOG_LEVEL="debug"
.\linkedbot.exe search --title "Software Engineer"
```

Or create a `.env` file with:
```
LINKEDBOT_LOG_LEVEL=debug
```

### Getting Help

1. Check screenshot files created during errors (search_fail.png, connect_button_fail.png, etc.)
2. Check HTML dumps (search_fail_full.html) to understand page structure
3. Review logs for specific error messages
4. LinkedIn UI changes frequently - selectors may need updates

### Known Limitations

- LinkedIn frequently changes their HTML structure, requiring selector updates
- Aggressive automation may trigger LinkedIn's anti-bot measures
- Some profiles have restricted visibility/actions
- Checkpoint/verification may trigger randomly
- Daily limits are enforced by LinkedIn's terms of service
