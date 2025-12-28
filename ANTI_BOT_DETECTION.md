# Anti-Bot Detection Strategy

## Overview
This LinkedIn automation bot implements **10+ sophisticated stealth techniques** to simulate authentic human behavior patterns and mask automation signatures, making it virtually indistinguishable from manual LinkedIn usage.

---

## Implemented Techniques

### 1. ✅ Human-like Mouse Movement (MANDATORY)
**Implementation:** Cubic Bézier curves with variable speed, natural overshoot, and micro-corrections

**Features:**
- **Cubic Bézier curves** instead of straight lines
- **Variable speed** - slower at start/end, faster in middle (ease-in-out)
- **Natural overshoot** - 30% chance of overshooting target and correcting
- **Micro-corrections** - small adjustments after reaching target
- **Jitter** - ±1-2px random variance throughout movement
- **Distance-aware** - more steps for longer distances

**Location:** `internal/stealth/stealth.go` - `MoveMouseHumanLike()`

---

### 2. ✅ Randomized Timing Patterns (MANDATORY)
**Implementation:** Gaussian distribution delays mimicking human cognitive processing

**Features:**
- **Gaussian sleep patterns** - delays cluster around mean (more realistic than uniform)
- **Context-aware delays:**
  - Slower at typing start (thinking time)
  - Pauses at punctuation (,. etc)
  - Variable scroll speed
  - Random "reading" pauses
- **Occasional breaks** - 15% chance of 3-8 second pause
- **Think time** before actions (mean 1.4s, stddev 600ms)

**Location:** `internal/stealth/stealth.go` - `SleepGaussian()`, `ThinkTime()`, `TakeBreak()`

---

### 3. ✅ Browser Fingerprint Masking (MANDATORY)
**Implementation:** Comprehensive fingerprint randomization and automation flag hiding

**Features:**
- **User agent randomization** - 5 latest realistic UAs
- **Platform consistency** - UA matches navigator.platform
- **Viewport randomization** - realistic window sizes
- **navigator.webdriver** removed
- **chrome object** mocked
- **Plugin array** populated with realistic plugins
- **Languages** set to realistic values
- **Hardware concurrency** randomized (4-12 cores)
- **Device memory** set to 8GB
- **Permission API** overridden
- **Timezone** consistent

**Location:** `internal/browser/browser.go` - `init()`, `getStealthScript()`

---

### 4. ✅ Realistic Typing Behavior
**Implementation:** Variable typing speed with typos and corrections

**Features:**
- **Keyboard-proximity typos** - 2% error rate with realistic nearby keys
- **Backspace corrections** - immediate correction after typo
- **Variable speed:**
  - 40-100ms at start (thinking)
  - 25-85ms in middle (flow)
  - 60-120ms at punctuation
  - 35-95ms after spaces
- **Gaussian noise** added to all delays
- **Occasional long pauses** - 5% chance of 300ms think time

**Location:** `internal/stealth/stealth.go` - `TypeHumanLike()`, `randomNearbyRune()`

---

### 5. ✅ Natural Scrolling Patterns
**Implementation:** Multi-step scrolling with reading pauses

**Features:**
- **Variable scroll distances** - 300-800px
- **Chunked scrolling** - 30% chance of scrolling in multiple chunks
- **Reading pauses** - 40% chance of 1.2s pause mid-scroll
- **Reverse scrolling** - 40% chance of scrolling back up (re-reading)
- **Gaussian timing** between scroll actions

**Location:** `internal/stealth/stealth.go` - `ScrollHumanLike()`

---

### 6. ✅ Canvas Fingerprint Randomization
**Implementation:** Minimal noise injection to prevent canvas fingerprinting

**Features:**
- **Pixel-level noise** - 0.1% of pixels modified by ±1
- **Consistent per session** - fingerprint stable within session
- **Undetectable** - noise too small to affect visual appearance

**Location:** `internal/browser/browser.go` - `getStealthScript()` (canvas toDataURL override)

---

### 7. ✅ WebGL Fingerprint Masking
**Implementation:** Override vendor/renderer strings

**Features:**
- **UNMASKED_VENDOR_WEBGL** → "Intel Inc."
- **UNMASKED_RENDERER_WEBGL** → "Intel Iris OpenGL Engine"
- Prevents GPU fingerprinting

**Location:** `internal/browser/browser.go` - `getStealthScript()` (WebGL overrides)

---

### 8. ✅ Screen Dimension Consistency
**Implementation:** Consistent screen/viewport relationship

**Features:**
- **Screen dimensions** always larger than viewport
- **availHeight** accounts for taskbar
- **Realistic ratios** maintained

**Location:** `internal/browser/browser.go` - `getStealthScript()` (screen object overrides)

---

### 9. ✅ Mouse Idle Movement
**Implementation:** Subtle mouse wandering when not clicking

**Features:**
- **30% activation** chance during pauses
- **Small movements** - 2-4 micro-movements
- **Safe zones** - stays away from edges
- Simulates human fidgeting

**Location:** `internal/stealth/stealth.go` - `MouseIdleMovement()`

---

### 10. ✅ Connection Information Masking
**Implementation:** Realistic network characteristics

**Features:**
- **effectiveType** → "4g"
- **downlink** → 10 Mbps
- **rtt** → 50ms
- **saveData** → false

**Location:** `internal/browser/browser.go` - `getStealthScript()` (connection override)

---

### 11. ✅ Battery API Masking
**Implementation:** Hide battery status to prevent fingerprinting

**Features:**
- **charging** → always true
- **level** → 100%
- Prevents battery-based device tracking

**Location:** `internal/browser/browser.go` - `getStealthScript()` (battery override)

---

### 12. ✅ Enhanced Click Behavior
**Implementation:** Realistic click with mouse movement, hover, and timing

**Features:**
- **Move to random point** within element bounds (not center)
- **Separate mousedown/mouseup** events
- **Human reaction time** - 30-90ms between down and up
- **Pre-click pause** - 50-150ms after movement

**Location:** `internal/stealth/stealth.go` - `ClickHumanLike()`

---

## Detection Evasion Summary

| Technique Category | Methods Implemented | Detection Risk |
|-------------------|---------------------|----------------|
| Mouse Behavior | Bézier curves, overshoot, micro-corrections, idle movement | Very Low |
| Timing Patterns | Gaussian delays, context-aware pauses, breaks | Very Low |
| Fingerprinting | UA, viewport, WebGL, Canvas, screen, plugins | Very Low |
| Input Behavior | Variable typing, typos, chunked scrolling | Very Low |
| API Masking | navigator.webdriver, chrome object, battery, connection | Very Low |

---

## Usage

The stealth mechanisms are **automatically applied** when the bot runs. No additional configuration required.

All techniques work together to create a behavioral profile indistinguishable from human LinkedIn usage.

---

## Testing

To verify stealth effectiveness:

1. **Bot detection sites:**
   - Visit https://bot.sannysoft.com/
   - Visit https://arh.antoinevastel.com/bots/areyouheadless
   - All checks should pass ✓

2. **LinkedIn behavior:**
   - Monitor for unusual connection patterns
   - Check if requests are being throttled
   - Verify no security challenges appear

---

## Maintenance

The stealth mechanisms are designed to be low-maintenance:

- **User agents** - Update quarterly with latest Chrome/Firefox versions
- **Fingerprints** - Randomization ensures uniqueness
- **Timing** - Gaussian patterns don't need tuning

Last updated: December 2025