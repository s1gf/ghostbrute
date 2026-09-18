# ghostbrute

Evasive Kerberos password spraying with traffic-shaped timing.

A fork of [kerbrute](https://github.com/ropnop/kerbrute) by [@ropnop](https://github.com/ropnop), adding CaptainCredz-style traffic shaping, cache/resume, and a dedicated `spraycampaign` mode for long-running red team engagements.

## Why ghostbrute?

Kerbrute's Kerberos pre-auth approach is already stealthier than SMB or LDAP spraying — it generates event 4768/4771 instead of the heavily-monitored 4625, and gokrb5's AS-REQ packets are indistinguishable from a real Windows client on the wire. The only thing that burns it is **speed**: a burst of AS-REQs from one IP at 3 AM on a Sunday is trivially flagged by any rate-based detection.

ghostbrute solves this by shaping spray traffic to match normal business-hour authentication patterns — spraying faster during peak hours, slower at night, near-silent on weekends — so the requests blend into legitimate AD noise.

## Features

| Feature | Description |
|---|---|
| **spraycampaign** | Spray a password list across a user list, one password at a time |
| **Traffic-shaped timing** | Hour-of-day and day-of-week factors, daily speed ramp, configurable jitter |
| **Cache/resume** | JSON state file — Ctrl+C and restart, picks up where it left off |
| **Lockout safety** | `--max-per-user` caps attempts per user; `--safe` aborts on any lockout |
| **Fatal error handling** | Network errors and lockouts abort the campaign instead of spraying into the void |
| **Interruptible** | All delays respond to Ctrl+C immediately — no hanging on long sleeps |
| **Secure by default** | Cache and log files written with 0600 permissions |

Plus all original kerbrute commands: `userenum`, `passwordspray`, `bruteuser`, `bruteforce`.

## Installation

Grab a binary from the [releases page](https://github.com/ghostbrute/ghostbrute/releases), or build from source:

```bash
git clone https://github.com/ghostbrute/ghostbrute.git
cd ghostbrute
make all
ls dist/
```

Builds for Linux, Windows, and macOS (amd64 + arm64). Single static binary, no dependencies.

## Quick Start

```bash
# Basic spray campaign with default timing (business hours curve)
./ghostbrute spraycampaign -d contoso.local --dc 10.0.0.1 users.txt passwords.txt

# With custom timing profile and cache
./ghostbrute spraycampaign -d contoso.local --dc 10.0.0.1 \
  --timing-config timing.json \
  --cache-file engagement.cache \
  --safe -v \
  users.txt passwords.txt

# Limit to 3 attempts per user + try username as password
./ghostbrute spraycampaign -d contoso.local --dc 10.0.0.1 \
  --max-per-user 3 \
  --user-as-pass \
  --timing-config timing.json \
  users.txt passwords.txt
```

## Commands

### spraycampaign (new)

The primary mode. Sprays one password at a time across all users, with traffic-shaped delays between each request. Supports cache/resume and timing profiles.

```
ghostbrute spraycampaign [flags] <username_wordlist> <password_wordlist>

Flags:
    --timing-config string   JSON timing config file for traffic-shaped delays
    --cache-file string      Cache file for resume support (default: ghostbrute.cache)
    --max-per-user int       Max password attempts per user (0 = unlimited)
    --user-as-pass           Also try each username as its own password
```

**Workflow:**
1. If `--user-as-pass` is set, tries every username as its own password first (Phase 0)
2. Sprays password 1 across all users, respecting timing delays
3. Waits between password sweeps (3x the computed delay)
4. Sprays password 2, and so on
5. Pauses automatically during off-hours (factor < 0.05) and resumes when the activity window opens
6. On Ctrl+C, saves state and exits — restart with the same `--cache-file` to resume

### userenum

Enumerate valid domain usernames via Kerberos. Does not cause login failures or lockouts.

```bash
./ghostbrute userenum -d contoso.local --dc 10.0.0.1 usernames.txt
```

### passwordspray

Test a single password against a list of users (original kerbrute behavior).

```bash
./ghostbrute passwordspray -d contoso.local --dc 10.0.0.1 users.txt 'Spring2026!'
```

### bruteuser

Bruteforce a single user's password from a wordlist. Only use if there is no lockout policy.

```bash
./ghostbrute bruteuser -d contoso.local --dc 10.0.0.1 passwords.txt jsmith
```

### bruteforce

Test username:password combos from a file or stdin.

```bash
./ghostbrute bruteforce -d contoso.local --dc 10.0.0.1 combos.txt
cat combos.txt | ./ghostbrute bruteforce -d contoso.local -
```

## Timing Configuration

The timing engine controls how fast ghostbrute sprays based on the time of day and day of the week. Create a JSON config file:

```json
{
  "utc_offset": 2,
  "base_delay": 5.0,
  "daily_speedup": 1.25,
  "initial_speed": 0.5,
  "jitter_min": 1.0,
  "jitter_max": 3.0,
  "hours_factor": {
    "0": 0.1, "1": 0.1, "2": 0.1, "3": 0.1,
    "4": 0.1, "5": 0.1, "6": 0.2, "7": 0.5,
    "8": 1.0, "9": 1.0, "10": 0.8, "11": 0.4,
    "12": 0.6, "13": 0.8, "14": 0.5, "15": 0.5,
    "16": 0.5, "17": 0.5, "18": 0.6, "19": 0.3,
    "20": 0.2, "21": 0.1, "22": 0.1, "23": 0.1
  },
  "days_factor": {
    "mon": 1.0, "tue": 1.0, "wed": 1.0, "thu": 1.0, "fri": 1.0,
    "sat": 0.1, "sun": 0.1
  }
}
```

An example config is included: [`timing.example.json`](timing.example.json)

### Timing fields

| Field | Description | Default |
|---|---|---|
| `utc_offset` | Target timezone as UTC offset (e.g. `2` for CEST, `-5` for EST) | `0` |
| `base_delay` | Base delay in seconds between requests at factor 1.0 | `5.0` |
| `initial_speed` | Starting speed multiplier | `1.0` |
| `daily_speedup` | Multiplier applied per day of running (ramps up over time) | `1.0` |
| `jitter_min` | Minimum random jitter added to each delay (seconds) | `0` |
| `jitter_max` | Maximum random jitter added to each delay (seconds) | `0` |
| `hours_factor` | Speed factor per hour (0-23). `1.0` = full speed, `0.1` = 10x slower | Business hours curve |
| `days_factor` | Speed factor per day (mon-sun). `1.0` = full speed, `0.1` = 10x slower | Weekdays full, weekends slow |

### How delay is computed

```
effective_delay = base_delay / (hour_factor * day_factor * speed_multiplier) + jitter
```

Where `speed_multiplier = initial_speed * daily_speedup ^ days_running` (capped at 10x).

The minimum effective delay is 100ms regardless of configuration.

**Example:** At 9 AM on a Tuesday with the default config:
- `hour_factor = 1.0`, `day_factor = 1.0`, `speed = 1.0`
- `delay = 5.0 / (1.0 * 1.0 * 1.0) = 5 seconds` + jitter

At 2 AM on a Sunday:
- `hour_factor = 0.1`, `day_factor = 0.1`, `speed = 1.0`
- `delay = 5.0 / (0.1 * 0.1 * 1.0) = 500 seconds` → pauses (factor < 0.05)

### Off-hours pausing

When `hour_factor * day_factor < 0.05`, the campaign pauses entirely and polls every 30 seconds until the factors rise above the threshold. This happens during nighttime on weekends with the default config.

## Cache / Resume

ghostbrute writes a JSON cache file after every attempt:

```json
{
  "results": [
    {"username": "jsmith", "password": "Spring2026!", "success": false, "error": "Invalid password", "timestamp": "2026-09-18T09:15:32Z"},
    {"username": "jdoe", "password": "Spring2026!", "success": true, "timestamp": "2026-09-18T09:15:37Z"}
  ]
}
```

On restart with the same `--cache-file`, already-tried username:password pairs are skipped automatically.

The cache file is written with `0600` permissions (owner-only read/write).

## Global Flags

```
-d, --domain string      The full domain to use (e.g. contoso.com)
    --dc string          The location of the Domain Controller (KDC) to target. If blank, will lookup via DNS
-o, --output string      File to write logs to (written with 0600 permissions). Optional.
-v, --verbose            Log failures and errors
    --safe               Safe mode. Will abort if any user comes back as locked out.
-t, --threads int        Threads to use (default 10)
    --delay int          Delay in milliseconds between each attempt. Forces single thread if set
    --downgrade          Force downgraded encryption type (arcfour-hmac-md5)
    --hash-file string   File to save AS-REP hashes to (if any captured)
```

## Detection and OPSEC Notes

**What makes ghostbrute harder to detect:**
- Kerberos pre-auth generates event 4768/4771, not the commonly-monitored 4625
- gokrb5's AS-REQ packets match real Windows Kerberos clients on the wire — no known byte-level signatures
- Traffic shaping blends requests into normal authentication patterns
- Off-hours pausing avoids anomalous nighttime/weekend activity

**What can still detect it:**
- Suricata/NDR rules that threshold on AS-REQ volume from a single source IP (default rule: 10+ in 30s — ghostbrute's timing engine keeps you well below this)
- SIEM correlation of 4768 events across many users from one source
- Anomalous `ClientAdvertizedEncryptionTypes` in event 4768 on Server 2016+ with 2025+ cumulative updates

**WARNING:** Failed Kerberos pre-authentication counts as a failed login and WILL lock out accounts. Always use `--safe` and `--max-per-user` on real engagements.

## Credits

- [kerbrute](https://github.com/ropnop/kerbrute) by [Ronnie Flathers (@ropnop)](https://github.com/ropnop) — the foundation this is built on
- [gokrb5](https://github.com/jcmturner/gokrb5) by jcmturner — pure Go Kerberos implementation
- [CaptainCredz](https://github.com/synacktiv/captaincredz) by Synacktiv — inspiration for the timing engine and traffic shaping concept
- [deadjakk](https://github.com/deadjakk) — original `spraycampaign` concept in kerbrute [PR #41](https://github.com/ropnop/kerbrute/pull/41)

## License

Same as kerbrute — see [LICENSE](LICENSE).
