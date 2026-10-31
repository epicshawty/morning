// Command morning prints a short daily briefing: weather, world news,
// tech headlines and one thing that happened on this day.
//
// https://github.com/epicshawty/morning
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/epicshawty/morning/internal/cache"
	"github.com/epicshawty/morning/internal/config"
	"github.com/epicshawty/morning/internal/facts"
	"github.com/epicshawty/morning/internal/fetch"
	"github.com/epicshawty/morning/internal/news"
	"github.com/epicshawty/morning/internal/ui"
	"github.com/epicshawty/morning/internal/weather"
)

// version is injected at build time: -ldflags "-X main.version=v1.0.0"
var version = "dev"

const (
	cacheTTL = 10 * time.Minute
	timeout  = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "morning: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		cityFlag     = flag.String("city", "", "city to report on (default: config, then IP lookup)")
		unitsFlag    = flag.String("units", "", "temperature units: c or f")
		sectionsFlag = flag.String("sections", "", "comma list of: weather,world,tech,fact")
		configFlag   = flag.String("config", "", "config file path")
		colorFlag    = flag.String("color", "auto", "colour: auto, always, never")
		jsonFlag     = flag.Bool("json", false, "emit JSON")
		plainFlag    = flag.Bool("plain", false, "force plain text")
		limitFlag    = flag.Int("limit", 0, "headlines per section (1-20)")
		widthFlag    = flag.Int("width", 0, "output width (default: terminal or 76)")
		cacheFlag    = flag.Bool("no-cache", false, "bypass the on-disk cache")
		versionFlag  = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Printf("morning %s\n", version)
		return nil
	}
	fetch.Version = version

	cfg, cfgErr := config.Load(*configFlag)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "morning: ignoring bad config: %v\n", cfgErr)
	}

	// CLI beats config, config beats defaults.
	if *cityFlag != "" {
		cfg.City = *cityFlag
	}
	if *unitsFlag != "" {
		cfg.Units = strings.ToLower(*unitsFlag)
	}
	if *limitFlag > 0 {
		cfg.Limit = *limitFlag
	}
	if *sectionsFlag != "" {
		cfg.Sections = splitList(*sectionsFlag)
	}

	units := "c"
	if cfg.Units == "f" {
		units = "f"
	}
	limit := cfg.Limit
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	want := sections(cfg.Sections)

	tty := isTerminal(os.Stdout)
	color := false
	switch *colorFlag {
	case "always":
		color = true
	case "never":
		color = false
	default:
		color = tty && os.Getenv("NO_COLOR") == ""
	}
	if *plainFlag || *jsonFlag {
		color = false
	}
	if !color {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	width := *widthFlag
	if width <= 0 {
		width = envWidth()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !*cacheFlag {
		fetch.UseCache(cache.Open(cacheTTL))
	}

	now := time.Now()
	greeting := greet(now)

	// Fetch everything concurrently; each section may fail on its own.
	var (
		wg        sync.WaitGroup
		wx        *weather.Report
		world     []news.Item
		tech      []news.Item
		factsList []facts.Fact
	)

	if want["weather"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := loadWeather(ctx, cfg.City)
			if err != nil {
				errWeather = err
				return
			}
			wx = &r
		}()
	}
	if want["world"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := news.World(ctx, limit)
			world, errWorld = list, err
		}()
	}
	if want["tech"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := news.Tech(ctx, limit)
			tech, errTech = list, err
		}()
	}
	if want["fact"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := facts.OnThisDay(ctx, now)
			factsList, errFacts = list, err
		}()
	}
	wg.Wait()

	if want["weather"] && wx == nil && errWeather != nil {
		// A briefing with no weather at all is a failed briefing.
		if !want["world"] && !want["tech"] && !want["fact"] {
			return fmt.Errorf("weather: %w", errWeather)
		}
	}

	if *jsonFlag {
		out := ui.JSON{
			GeneratedAt: now.Format(time.RFC3339),
			Greeting:    greeting,
			Version:     version,
			World:       world,
			Tech:        tech,
			OnThisDay:   factsList,
		}
		if wx != nil {
			out.Weather = &ui.WeatherOut{
				Report: wx,
				High:   weather.Value(wx.HighC, units),
				Low:    weather.Value(wx.LowC, units),
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	return ui.Render(os.Stdout, ui.Data{
		Now:        now,
		Greeting:   greeting,
		Weather:    wx,
		WeatherErr: errWeather,
		World:      world,
		WorldErr:   errWorld,
		Tech:       tech,
		TechErr:    errTech,
		Facts:      factsList,
		FactsErr:   errFacts,
		Opts: ui.Options{
			Color: color, Width: width, Units: units, Limit: limit, Version: version,
		},
	})
}

// Section errors are package-level so the fetch goroutines stay small; each is
// written by exactly one goroutine and read only after wg.Wait.
var (
	errWeather, errWorld, errTech, errFacts error
)

// loadWeather resolves a location (config city, else IP) and fetches conditions.
func loadWeather(ctx context.Context, city string) (weather.Report, error) {
	var (
		place weather.Place
		err   error
	)
	if city != "" {
		place, err = weather.Geocode(ctx, city)
		if err != nil {
			return weather.Report{}, err
		}
	} else {
		place, err = weather.LocateIP(ctx)
		if err != nil {
			return weather.Report{}, fmt.Errorf("could not auto-detect location (pass --city): %w", err)
		}
	}
	return weather.Fetch(ctx, place)
}

func greet(t time.Time) string {
	switch h := t.Hour(); {
	case h < 5:
		return "Still up?"
	case h < 12:
		return "Good morning"
	case h < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}

func sections(list []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range list {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "weather", "world", "tech", "fact":
			out[strings.ToLower(strings.TrimSpace(s))] = true
		case "news":
			out["world"] = true
		case "facts", "onthisday":
			out["fact"] = true
		}
	}
	if len(out) == 0 {
		for _, s := range config.Default().Sections {
			out[s] = true
		}
	}
	return out
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func envWidth() int {
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 40 && n <= 200 {
			return n
		}
	}
	return 76
}

func usage() {
	fmt.Fprintf(os.Stderr, `morning %s — a small daily briefing

Usage:
  morning [flags]

Flags:
  -city string      city to report on (default: config, then IP lookup)
  -units c|f        temperature units (default c)
  -sections list    any of weather,world,tech,fact (default all)
  -limit n          headlines per section, 1-20 (default 5)
  -json             emit JSON
  -plain            force plain text, no colour
  -color mode       auto|always|never (default auto)
  -width n          output width for wrapping
  -no-cache         bypass the 10 minute cache
  -config path      config file (default %s)
  -version          print version

Examples:
  morning
  morning -city Tokyo -units c
  morning -sections weather,tech
  morning -json | jq .weather
  morning > ~/today.txt

Config: %s
`, version, config.DefaultPath(), config.DefaultPath())
}
