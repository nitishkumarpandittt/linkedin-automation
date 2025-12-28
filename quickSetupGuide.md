# Quick Start Guide

## Prerequisites

1. **Go 1.22+** installed
2. **Google Chrome** or Chromium browser
3. **LinkedIn account** with valid credentials

## Setup (First Time)

### Step 1: Download Dependencies

```bash
go mod tidy
```

### Step 2: Build the Application

**Windows:**
```powershell
go build -o linkedbot.exe .\cmd\linkedbot
```

**Linux/Mac:**
```bash
go build -o linkedbot ./cmd/linkedbot
```

### Step 3: Configure

1. **Create .env file** from template:
   ```bash
   # Windows
   Copy-Item .env.example .env
   
   # Linux/Mac
   cp .env.example .env
   ```

2. **Edit .env** and add your LinkedIn credentials:
   ```env
   LINKEDIN_EMAIL=your-email@example.com
   LINKEDIN_PASSWORD=your-password
   ```

3. **Create config.yaml** from template:
   ```bash
   # Windows
   Copy-Item config.example.yaml config.yaml
   
   # Linux/Mac
   cp config.example.yaml config.yaml
   ```

4. **(Optional)** Edit `config.yaml` to customize:
   - Search defaults
   - Daily limits
   - Stealth settings
   - Message templates

### Step 4: Verify Setup

**Windows:**
```powershell
.\test-setup.ps1
```

**Linux/Mac:**
```bash
chmod +x test-setup.sh
./test-setup.sh
```

Fix any issues marked with ✗ before proceeding.

## Basic Usage

### 1. Login to LinkedIn

First, establish a session:

```bash
# Windows
.\linkedbot.exe login

# Linux/Mac
./linkedbot login
```

**Expected output:**
```
{"app":"linkedbot","level":"INFO","msg":"login successful"}
{"app":"linkedbot","level":"INFO","msg":"done"}
```

### 2. Search for Profiles

Search and collect target profiles:

```bash
# Windows
.\linkedbot.exe search --title "Software Engineer" --location "India" --limit 50

# Linux/Mac
./linkedbot search --title "Software Engineer" --location "India" --limit 50
```

**Parameters:**
- `--title`: Job title filter
- `--company`: Company name filter
- `--location`: Location filter
- `--keywords`: Additional keywords
- `--limit`: Max profiles to collect (default from config)

### 3. Send Connection Requests

Send connection requests to collected profiles:

```bash
# Windows
.\linkedbot.exe send-connections --limit 20

# Linux/Mac
./linkedbot send-connections --limit 20
```

The bot will:
- Send personalized connection requests
- Add custom notes from templates
- Respect daily limits
- Track sent connections in database

### 4. Send Follow-Up Messages

After connections are accepted, send follow-up messages:

```bash
# Windows
.\linkedbot.exe send-messages --limit 30

# Linux/Mac
./linkedbot send-messages --limit 30
```

The bot will:
- Check for newly accepted connections
- Send personalized follow-up messages
- Respect daily limits
- Track sent messages

### 5. Run Full Automation

Run all steps in sequence:

```bash
# Windows
$env:RUN_SEARCH="1"
$env:RUN_CONNECT="1"
$env:RUN_MESSAGE="1"
.\linkedbot.exe run-all

# Linux/Mac
export RUN_SEARCH=1
export RUN_CONNECT=1
export RUN_MESSAGE=1
./linkedbot run-all
```

## Customization

### Modify Search Criteria

Edit `config.yaml`:

```yaml
search:
  defaults:
    title: "Product Manager"
    company: "Google"
    location: "United States"
    keywords: "B2B SaaS"
```

### Customize Connection Note Template

Edit `config.yaml`:

```yaml
templates:
  connection_note_template: "Hi {{Name}}, I noticed your work at {{Company}}. Let's connect!"
```

Available variables:
- `{{Name}}` - Profile name
- `{{Company}}` - Company name
- `{{Title}}` - Job title/headline

### Adjust Daily Limits

Edit `config.yaml`:

```yaml
limits:
  max_connections_per_day: 30
  max_messages_per_day: 50
  max_profiles_per_search: 100
```

### Enable Debug Logging

Add to `.env`:

```env
LINKEDBOT_LOG_LEVEL=debug
```

## Common Issues

### "Checkpoint/Verification" Error

**Solution:** Login manually to LinkedIn from the same machine first, complete any verification, then try the bot.

### "No Links Found"

**Solution:** 
1. LinkedIn may have changed their HTML - check for updates
2. Try broader search criteria (remove filters)
3. Wait 30 minutes and try again (rate limiting)

### "Connection Button Not Found"

**Solution:**
1. Profile may not allow connections (some have "Follow" only)
2. You may already be connected
3. LinkedIn might be temporarily blocking actions

### Database Issues

**Solution:** Delete and recreate:
```bash
# Windows
del linkedbot.db
.\linkedbot.exe login

# Linux/Mac
rm linkedbot.db
./linkedbot login
```

## Safety Tips

1. **Start Slow:** Begin with small limits (10-20/day) and gradually increase
2. **Use Realistic Delays:** Don't decrease delay values in config
3. **Stay Within Active Hours:** Configure `active_start` and `active_end` to business hours
4. **Don't Run 24/7:** Take breaks, don't automate continuously
5. **Monitor Your Account:** Check LinkedIn regularly for any warnings

## Next Steps

- Review full [README.md](README.md) for architecture details
- Adjust stealth settings in `config.yaml`
- Customize message templates
- Set up scheduling (cron/Task Scheduler) for automated runs

## Getting Help

1. Run setup test: `./test-setup.sh` or `.\test-setup.ps1`
2. Check screenshot files created during errors
3. Enable debug logging: `LINKEDBOT_LOG_LEVEL=debug`
4. Review logs for specific error messages