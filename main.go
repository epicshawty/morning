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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // embed the zone database so -city works on any OS

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

// buildVersion reports the tag baked in by ldflags, falling back to the module
// version recorded in the build info. `go install module@v1.2.3` passes no
// ldflags, so without this the documented install path reports "dev".
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && isReleaseTag(bi.Main.Version) {
		return bi.Main.Version
	}
	return version
}

// isReleaseTag accepts v1.2.3 and rejects the pseudo-versions Go stamps into
// local builds (v1.2.4-0.20260101...-dirty), which are noise in -version.
func isReleaseTag(v string) bool {
	if !strings.HasPrefix(v, "v") {
		return false
	}
	parts := strings.Split(v[1:], ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

const (
	cacheTTL = 10 * time.Minute
	timeout  = 20 * time.Second
)

func main() {
	version = buildVersion()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "morning: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		cityFlag     = flag.String("city", "", "city to report on (default: config, then IP lookup)")
		unitsFlag    = flag.String("units", "", "temperature units: c, f or auto (default auto, follows the city)")
		sectionsFlag = flag.String("sections", "", "comma list of: weather,local,world,tech,fact")
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

	units := cfg.Units // "", "c" or "f"; empty means follow the city
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

	// Fetch everything concurrently; each section may fail on its own.
	var (
		wg         sync.WaitGroup
		wx         *weather.Report
		world      []news.Item
		local      []news.Item
		tech       []news.Item
		factsList  []facts.Fact
		localFacts []facts.Fact
	)

	if want["world"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			world, errWorld = news.World(ctx, limit)
		}()
	}
	if want["tech"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tech, errTech = news.Tech(ctx, limit)
		}()
	}

	// Weather, local news and local history all describe one place, so resolve it
	// once — while the two feeds above are already in flight.
	var place weather.Place
	if want["weather"] || want["local"] || want["fact"] {
		p, err := resolvePlace(ctx, cfg.City)
		if err != nil {
			errPlace = err
		} else {
			place = p
		}
	}

	if want["weather"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if errPlace != nil {
				errWeather = errPlace
				return
			}
			r, err := weather.Fetch(ctx, place)
			if err != nil {
				errWeather = err
				return
			}
			wx = &r
		}()
	}
	if want["local"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if errPlace != nil {
				errLocal = errPlace
				return
			}
			local, errLocal = news.Local(ctx, place.Name, limit)
		}()
	}
	if want["fact"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if list, err := facts.OnThisDay(ctx, now); err != nil {
				errFacts = err
			} else {
				factsList = list
			}
			// A city-specific list only exists on some days; when it does, it is
			// more interesting than the global one.
			if errPlace == nil && place.Name != "" {
				if lf, err := facts.Local(ctx, now, place.Lat, place.Lon, place.Name, 2); err == nil {
					localFacts = lf
				}
			}
		}()
	}
	wg.Wait()

	if want["weather"] && wx == nil && errWeather != nil {
		// A briefing with no weather at all is a failed briefing.
		if !want["world"] && !want["tech"] && !want["fact"] && !want["local"] {
			return fmt.Errorf("weather: %w", errWeather)
		}
	}

	// The briefing describes a place, so the clock, the greeting and the units
	// follow that place instead of this machine.
	cityNow := now
	var homeNow *time.Time
	if place.Name != "" {
		if loc := locationOf(place); loc != nil {
			cityNow = now.In(loc)
		}
		if cityNow.Format("-0700") != now.Format("-0700") {
			home := now.In(time.Local)
			homeNow = &home
		}
	}
	greeting := greet(cityNow)

	if units == "" {
		units = autoUnits(place)
	}

	// Local history beats the global list when today has something nearby.
	factCity := ""
	if len(localFacts) > 0 {
		factsList = localFacts
		factCity = place.Name
	}

	if *jsonFlag {
		out := ui.JSON{
			GeneratedAt: cityNow.Format(time.RFC3339),
			Greeting:    greeting,
			Version:     version,
			Units:       units,
			Local:       local,
			World:       world,
			Tech:        tech,
			OnThisDay:   factsList,
		}
		if factCity != "" {
			out.OnThisDayCity = factCity
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
		Now:        cityNow,
		HomeNow:    homeNow,
		Greeting:   greeting,
		Weather:    wx,
		WeatherErr: errWeather,
		Local:      local,
		LocalErr:   errLocal,
		LocalName:  place.Name,
		World:      world,
		WorldErr:   errWorld,
		Tech:       tech,
		TechErr:    errTech,
		Facts:      factsList,
		FactsErr:   errFacts,
		FactCity:   factCity,
		Opts: ui.Options{
			Color: color, Width: width, Units: units, Limit: limit, Version: version,
		},
	})
}

// fahrenheitCountries are the places that report temperature in °F. Everything
// else — including Canada and the UK — uses °C.
var fahrenheitCountries = map[string]bool{
	"US": true, "BS": true, "BZ": true, "KY": true,
	"PW": true, "FM": true, "MH": true, "LR": true,
}

// autoUnits picks the unit the reported place actually uses.
func autoUnits(p weather.Place) string {
	if fahrenheitCountries[strings.ToUpper(p.Country)] {
		return "f"
	}
	return "c"
}

// locationOf resolves a place's IANA zone. It embeds time/tzdata, so this works
// on a bare Windows or container host with no zoneinfo installed.
func locationOf(p weather.Place) *time.Location {
	if p.TZ != "" {
		if loc, err := time.LoadLocation(p.TZ); err == nil {
			return loc
		}
	}
	return nil
}

// Section errors are package-level so the fetch goroutines stay small; each is
// written by at most one goroutine and read only after wg.Wait.
var (
	errPlace, errWeather, errLocal, errWorld, errTech, errFacts error
)

// resolvePlace turns the configured city, or the public IP, into coordinates.
// Everything place-bound (weather, local news, local history, the clock) hangs
// off this one lookup.
func resolvePlace(ctx context.Context, city string) (weather.Place, error) {
	if city != "" {
		return weather.Geocode(ctx, city)
	}
	p, err := weather.LocateIP(ctx)
	if err != nil {
		return weather.Place{}, fmt.Errorf("could not auto-detect location (pass -city): %w", err)
	}
	return p, nil
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
		case "weather", "local", "world", "tech", "fact":
			out[strings.ToLower(strings.TrimSpace(s))] = true
		case "news":
			out["world"] = true
		case "city", "citynews":
			out["local"] = true
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
  -units c|f|auto   temperature units (default auto: follows the city)
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
