package timing

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
)

const maxSpeedMultiplier = 10.0

type TimingConfig struct {
	UTCOffset    float64            `json:"utc_offset"`
	DailySpeedup float64           `json:"daily_speedup"`
	InitialSpeed float64           `json:"initial_speed"`
	HoursFactor  map[string]float64 `json:"hours_factor"`
	DaysFactor   map[string]float64 `json:"days_factor"`
	JitterMin    float64            `json:"jitter_min"`
	JitterMax    float64            `json:"jitter_max"`
	BaseDelay    float64            `json:"base_delay"`
}

type Engine struct {
	config    TimingConfig
	startTime time.Time
}

func DefaultConfig() TimingConfig {
	return TimingConfig{
		UTCOffset:    0,
		DailySpeedup: 1.0,
		InitialSpeed: 1.0,
		BaseDelay:    5.0,
		JitterMin:    0,
		JitterMax:    0,
		HoursFactor: map[string]float64{
			"0": 0.1, "1": 0.1, "2": 0.1, "3": 0.1,
			"4": 0.1, "5": 0.1, "6": 0.2, "7": 0.5,
			"8": 1.0, "9": 1.0, "10": 0.8, "11": 0.4,
			"12": 0.6, "13": 0.8, "14": 0.5, "15": 0.5,
			"16": 0.5, "17": 0.5, "18": 0.6, "19": 0.3,
			"20": 0.2, "21": 0.1, "22": 0.1, "23": 0.1,
		},
		DaysFactor: map[string]float64{
			"mon": 1.0, "tue": 1.0, "wed": 1.0, "thu": 1.0, "fri": 1.0,
			"sat": 0.1, "sun": 0.1,
		},
	}
}

func LoadConfig(path string) (TimingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TimingConfig{}, fmt.Errorf("failed to read timing config: %w", err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return TimingConfig{}, fmt.Errorf("failed to parse timing config: %w", err)
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 5.0
	}
	if cfg.InitialSpeed <= 0 {
		cfg.InitialSpeed = 1.0
	}
	if cfg.DailySpeedup <= 0 {
		cfg.DailySpeedup = 1.0
	}
	if cfg.JitterMin < 0 {
		cfg.JitterMin = 0
	}
	if cfg.JitterMax < cfg.JitterMin {
		cfg.JitterMax = cfg.JitterMin
	}
	return cfg, nil
}

func NewEngine(config TimingConfig) *Engine {
	return &Engine{
		config:    config,
		startTime: time.Now(),
	}
}

func (e *Engine) targetTime() time.Time {
	offset := time.Duration(e.config.UTCOffset * float64(time.Hour))
	return time.Now().UTC().Add(offset)
}

func (e *Engine) daysSinceStart() float64 {
	return time.Since(e.startTime).Hours() / 24.0
}

func (e *Engine) hourFactor() float64 {
	hour := e.targetTime().Hour()
	key := fmt.Sprintf("%d", hour)
	if f, ok := e.config.HoursFactor[key]; ok {
		return f
	}
	return 0.5
}

func (e *Engine) dayFactor() float64 {
	weekday := e.targetTime().Weekday()
	dayNames := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	key := dayNames[weekday]
	if f, ok := e.config.DaysFactor[strings.ToLower(key)]; ok {
		return f
	}
	return 0.5
}

func (e *Engine) speedMultiplier() float64 {
	days := e.daysSinceStart()
	m := e.config.InitialSpeed * math.Pow(e.config.DailySpeedup, days)
	if m > maxSpeedMultiplier {
		return maxSpeedMultiplier
	}
	return m
}

func (e *Engine) ComputeDelay() time.Duration {
	hf := e.hourFactor()
	df := e.dayFactor()
	speed := e.speedMultiplier()

	combinedFactor := hf * df * speed
	if combinedFactor <= 0 {
		combinedFactor = 0.01
	}

	delaySeconds := e.config.BaseDelay / combinedFactor

	if e.config.JitterMax > e.config.JitterMin {
		jitter := e.config.JitterMin + rand.Float64()*(e.config.JitterMax-e.config.JitterMin)
		delaySeconds += jitter
	}

	if delaySeconds < 0.1 {
		delaySeconds = 0.1
	}

	return time.Duration(delaySeconds * float64(time.Second))
}

func (e *Engine) ShouldPause() bool {
	hf := e.hourFactor()
	df := e.dayFactor()
	return hf*df < 0.05
}

func (e *Engine) Status() string {
	t := e.targetTime()
	delay := e.ComputeDelay()
	return fmt.Sprintf("target_time=%s hour_factor=%.2f day_factor=%.2f speed=%.2f delay=%v",
		t.Format("15:04:05"),
		e.hourFactor(),
		e.dayFactor(),
		e.speedMultiplier(),
		delay.Round(time.Millisecond),
	)
}
