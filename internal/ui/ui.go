// Package ui renders the briefing in styled or plain form.
//
// Styled output uses ANSI colour and is only ever produced for a terminal.
// Plain output is pure ASCII so `morning | grep tech` behaves.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/epicshawty/morning/internal/facts"
	"github.com/epicshawty/morning/internal/news"
	"github.com/epicshawty/morning/internal/weather"
)

// Options controls layout and units.
type Options struct {
	Rich    bool // use the styled dashboard layout at all
	Color   bool // emit ANSI colour
	Links   bool // emit OSC 8 hyperlinks around headlines
	Width   int
	Units   string // "c" or "f"
	Limit   int
	Version string
}

// hyperlink wraps text in an OSC 8 terminal hyperlink. Terminals that do not
// understand it ignore the sequence, and it carries no width, so wrapping and
// alignment are unaffected.
func hyperlink(url, text string, on bool) string {
	if !on || url == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// Data is everything the current run managed to collect.
type Data struct {
	Now        time.Time
	HomeNow    *time.Time // the machine's clock, only when it differs from Now
	Greeting   string
	Weather    *weather.Report
	WeatherErr error
	Local      []news.Item
	LocalErr   error
	LocalName  string
	World      []news.Item
	WorldErr   error
	Tech       []news.Item
	TechErr    error
	Facts      []facts.Fact
	FactsErr   error
	FactCity   string // set when Facts were narrowed to a city
	Opts       Options
}

// JSON is the stable machine-readable shape of a run.
type JSON struct {
	GeneratedAt   string       `json:"generated_at"`
	Greeting      string       `json:"greeting"`
	Version       string       `json:"version"`
	Units         string       `json:"units"`
	Weather       *WeatherOut  `json:"weather,omitempty"`
	Error         string       `json:"weather_error,omitempty"`
	Local         []news.Item  `json:"local,omitempty"`
	World         []news.Item  `json:"world,omitempty"`
	Tech          []news.Item  `json:"tech,omitempty"`
	OnThisDay     []facts.Fact `json:"on_this_day,omitempty"`
	OnThisDayCity string       `json:"on_this_day_city,omitempty"`
}

// WeatherOut is the weather report with the requested unit pre-rendered.
type WeatherOut struct {
	*weather.Report
	High string `json:"high"`
	Low  string `json:"low"`
}

// colorProfile picks the richest profile the terminal advertises. lipgloss would
// otherwise guess from stdout, which is wrong the moment output is redirected.
func colorProfile() termenv.Profile {
	switch strings.ToLower(os.Getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return termenv.TrueColor
	}
	return termenv.ANSI256
}

var (
	cAccent = lipgloss.AdaptiveColor{Light: "#a15c00", Dark: "#ffb454"}
	cHead   = lipgloss.AdaptiveColor{Light: "#0a5cb8", Dark: "#79c0ff"}
	cMuted  = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#8a8a8a"}
	cBody   = lipgloss.AdaptiveColor{Light: "#1c1c1c", Dark: "#d8d8d8"}

	stGreet   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stDate    = lipgloss.NewStyle().Foreground(cBody)
	stPlace   = lipgloss.NewStyle().Foreground(cMuted)
	stSection = lipgloss.NewStyle().Bold(true).Foreground(cHead)
	stBody    = lipgloss.NewStyle().Foreground(cBody)
	stMuted   = lipgloss.NewStyle().Foreground(cMuted)
	stBullet  = lipgloss.NewStyle().Foreground(cAccent)
)

// Render writes the briefing to w.
func Render(w io.Writer, d Data) error {
	if !d.Opts.Rich {
		return renderPlain(w, d)
	}

	// Colour and hyperlinks are independent: a terminal with NO_COLOR still gets
	// clickable headlines, and a piped run gets neither.
	if d.Opts.Color {
		lipgloss.SetColorProfile(colorProfile())
	} else {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	width := d.Opts.Width
	if width < 44 {
		width = 76
	}
	// The body is indented by two columns; keep the rule inside the terminal.
	width -= 2

	var b strings.Builder

	// ── greeting ──────────────────────────────────────────────
	glyph := "☀"
	if h := d.Now.Hour(); h >= 18 || h < 5 {
		glyph = "☾"
	} else if h >= 12 {
		glyph = "◐"
	}
	fmt.Fprintf(&b, "%s  %s\n", stGreet.Render(glyph), stGreet.Render(d.Greeting))
	fmt.Fprintf(&b, "   %s\n", stDate.Render(dateLine(d)))

	if d.Weather != nil {
		if l := d.Weather.Place.Label(); l != "" {
			fmt.Fprintf(&b, "   %s\n", stPlace.Render(l))
		}
	}
	b.WriteString("\n")

	// ── weather ───────────────────────────────────────────────
	if d.Weather != nil {
		w := d.Weather
		b.WriteString(header("Weather", width))
		fmt.Fprintf(&b, "  %s · %s\n",
			stBody.Bold(true).Foreground(cAccent).Render(iconCell(w.Emoji)+w.Desc),
			stBody.Render(weather.Value(w.TempC, d.Opts.Units)),
		)
		fmt.Fprintf(&b, "  %s\n", stMuted.Render(fmt.Sprintf(
			"high %s · low %s · feels %s · rain %d%% · wind %s",
			weather.Temp(w.HighC, d.Opts.Units),
			weather.Temp(w.LowC, d.Opts.Units),
			weather.Temp(w.FeelsC, d.Opts.Units),
			w.Rain,
			wind(w.WindKPH, d.Opts.Units),
		)))
		if w.Sunrise != "" || w.Sunset != "" {
			fmt.Fprintf(&b, "  %s\n", stMuted.Render(fmt.Sprintf(
				"↑ %s   ↓ %s", w.Sunrise, w.Sunset)))
		}
		b.WriteString("\n")
	} else if d.WeatherErr != nil {
		b.WriteString(header("Weather", width))
		fmt.Fprintf(&b, "  %s\n\n", stMuted.Render("unavailable — "+short(d.WeatherErr)))
	}

	// ── local news ────────────────────────────────────────────
	if len(d.Local) > 0 || d.LocalErr != nil {
		b.WriteString(header(localTitle(d), width))
		b.WriteString(items(d.Local, d.LocalErr, width, d.Opts.Links))
		b.WriteString("\n")
	}

	// ── world news ────────────────────────────────────────────
	if len(d.World) > 0 || d.WorldErr != nil {
		b.WriteString(header("World", width))
		b.WriteString(items(d.World, d.WorldErr, width, d.Opts.Links))
		b.WriteString("\n")
	}

	// ── tech ──────────────────────────────────────────────────
	if len(d.Tech) > 0 || d.TechErr != nil {
		b.WriteString(header("Tech", width))
		b.WriteString(items(d.Tech, d.TechErr, width, d.Opts.Links))
		b.WriteString("\n")
	}

	// ── on this day ───────────────────────────────────────────
	if len(d.Facts) > 0 || d.FactsErr != nil {
		b.WriteString(header(factTitle(d), width))
		if len(d.Facts) == 0 {
			fmt.Fprintf(&b, "  %s\n\n", stMuted.Render("unavailable — "+short(d.FactsErr)))
		}
		n := d.Facts
		if len(n) > 2 {
			n = n[:2]
		}
		for _, f := range n {
			prefix := ""
			if f.Year > 0 {
				prefix = fmt.Sprint(f.Year) + " —"
			}
			b.WriteString(bullet(bulletSpec{prefix: prefix, text: f.Text, url: f.Link}, width, d.Opts.Links))
		}
		b.WriteString("\n")
	}

	fmt.Fprint(w, indent(strings.TrimRight(b.String(), "\n")+"\n", "  "))
	return nil
}

// localTitle names the city section after the place being reported on.
func localTitle(d Data) string {
	if d.LocalName != "" {
		return "Local · " + d.LocalName
	}
	return "Local"
}

// factTitle flags when the on-this-day list has been narrowed to a city.
func factTitle(d Data) string {
	if d.FactCity != "" {
		return "On this day in " + d.FactCity
	}
	return "On this day"
}

// iconCell pads a weather glyph out to a fixed column width.
//
// Terminal fonts draw these symbols almost to the edge of their cell, so a
// single space leaves the glyph looking welded to the word beside it. Reserving
// three cells guarantees visible separation whatever the font does.
func iconCell(glyph string) string {
	if glyph == "" {
		return ""
	}
	pad := 3 - lipgloss.Width(glyph)
	if pad < 1 {
		pad = 1
	}
	return glyph + strings.Repeat(" ", pad)
}

// dateLine renders the local date and time of the place being reported on,
// noting this machine's clock only when the two differ.
func dateLine(d Data) string {
	s := d.Now.Format("Monday, 2 January 2006 · 15:04") + " " + zoneLabel(d.Now)
	if d.HomeNow != nil {
		s += " (" + d.HomeNow.Format("15:04") + " your time)"
	}
	return s
}

// zoneLabel names the zone the way a person would read it.
//
// tzdata has no letter abbreviation for some zones and reports a bare offset
// instead — Asia/Dhaka is "+06", Asia/Kathmandu "+0545" — which reads as a stray
// number next to the clock. Those become "UTC+6" and "UTC+5:45".
func zoneLabel(t time.Time) string {
	name := t.Format("MST")
	if isNumericZone(name) {
		_, offset := t.Zone()
		sign := "+"
		if offset < 0 {
			sign, offset = "-", -offset
		}
		hours, minutes := offset/3600, (offset%3600)/60
		if minutes == 0 {
			return fmt.Sprintf("UTC%s%d", sign, hours)
		}
		return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
	}
	return name
}

// isNumericZone reports whether tzdata gave an offset ("+06", "-0330") rather
// than a name like "JST".
func isNumericZone(name string) bool {
	if len(name) < 2 || (name[0] != '+' && name[0] != '-') {
		return false
	}
	for _, r := range name[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func header(title string, width int) string {
	rule := strings.Repeat("─", max(0, width-len(title)-5))
	return fmt.Sprintf("%s %s\n", stSection.Render(strings.ToUpper(title)), stMuted.Render(rule))
}

func items(list []news.Item, err error, width int, links bool) string {
	if len(list) == 0 {
		return fmt.Sprintf("  %s\n", stMuted.Render("unavailable — "+short(err)))
	}
	var b strings.Builder
	for _, it := range list {
		tag := it.Source
		if it.Score > 0 {
			tag = fmt.Sprintf("%s · %d", it.Source, it.Score)
		}
		sp := bulletSpec{text: it.Title, url: it.URL}
		if tag != "" {
			sp.tag = "(" + tag + ")"
		}
		b.WriteString(bullet(sp, width, links))
	}
	return b.String()
}

// bulletSpec is one rendered bullet: optional dim prefix, the text, an optional
// dim trailing tag, and the URL it should link to.
type bulletSpec struct {
	prefix string
	text   string
	tag    string
	url    string
}

// bullet renders one wrapped bullet.
//
// prefix, when set, is dimmed at the start of the first line; tag, when set, is
// dimmed at the end of the last. Wrapping runs on the plain string before any
// styling, so ANSI sequences can never skew the measured width. The hyperlink is
// applied per line, after wrapping, so every line of a long headline is
// clickable.
func bullet(sp bulletSpec, width int, links bool) string {
	const gutter = 4 // two spaces of indent plus "· "
	avail := max(24, width-gutter)

	body := sp.text
	if sp.prefix != "" {
		body = sp.prefix + " " + sp.text
	}
	lines := wrap(body, avail)

	// \x00 marks the point where the dim tag begins.
	if sp.tag != "" {
		last := len(lines) - 1
		if utf8.RuneCountInString(lines[last])+1+utf8.RuneCountInString(sp.tag) <= avail {
			lines[last] += "\x00" + sp.tag
		} else {
			lines = append(lines, "\x00"+sp.tag)
		}
	}

	var b strings.Builder
	for i, ln := range lines {
		var line strings.Builder
		if i == 0 {
			line.WriteString("  " + stBullet.Render("·") + " ")
		} else {
			line.WriteString("    ")
		}
		if i == 0 && sp.prefix != "" && strings.HasPrefix(ln, sp.prefix) {
			line.WriteString(stMuted.Render(sp.prefix))
			ln = strings.TrimPrefix(ln, sp.prefix)
		}
		if idx := strings.IndexByte(ln, 0x00); idx >= 0 {
			if idx > 0 {
				line.WriteString(stBody.Render(ln[:idx]) + " ")
			}
			line.WriteString(stMuted.Render(ln[idx+1:]))
		} else {
			line.WriteString(stBody.Render(ln))
		}
		b.WriteString(hyperlink(sp.url, line.String(), links))
		b.WriteString("\n")
	}
	return b.String()
}

// wrap breaks text on spaces to fit width.
func wrap(s string, width int) []string {
	if utf8.RuneCountInString(s) <= width {
		return []string{s}
	}
	var out []string
	var cur strings.Builder
	for _, word := range strings.Fields(s) {
		wl := utf8.RuneCountInString(word)
		if cur.Len() > 0 && utf8.RuneCountInString(cur.String())+1+wl > width {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(word)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func indent(s, pad string) string {
	var b strings.Builder
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString(pad + ln + "\n")
	}
	return b.String()
}

func wind(kph float64, units string) string {
	if units == "f" {
		return fmt.Sprintf("%.0f mph", kph/1.609344)
	}
	return fmt.Sprintf("%.0f km/h", kph)
}

func short(err error) string {
	if err == nil {
		return "no data"
	}
	s := err.Error()
	if len(s) > 90 {
		s = s[:87] + "..."
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─────────────────────────────────────────────────────────────
// plain renderer
// ─────────────────────────────────────────────────────────────

func renderPlain(w io.Writer, d Data) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.Greeting)
	fmt.Fprintf(&b, "%s\n\n", dateLine(d))

	if d.Weather != nil {
		wx := d.Weather
		if l := wx.Place.Label(); l != "" {
			fmt.Fprintf(&b, "%s\n", l)
		}
		fmt.Fprintf(&b, "%s\n", headerPlain("WEATHER"))
		fmt.Fprintf(&b, "  %s, %s (feels like %s)\n", wx.Desc,
			weather.Value(wx.TempC, d.Opts.Units), weather.Value(wx.FeelsC, d.Opts.Units))
		fmt.Fprintf(&b, "  High %s, low %s, rain %d%%, wind %s\n",
			weather.Value(wx.HighC, d.Opts.Units), weather.Value(wx.LowC, d.Opts.Units),
			wx.Rain, wind(wx.WindKPH, d.Opts.Units))
		if wx.Sunrise != "" || wx.Sunset != "" {
			fmt.Fprintf(&b, "  Sunrise %s, sunset %s\n", wx.Sunrise, wx.Sunset)
		}
		b.WriteString("\n")
	} else if d.WeatherErr != nil {
		fmt.Fprintf(&b, "%s\n  unavailable: %s\n\n", headerPlain("WEATHER"), short(d.WeatherErr))
	}

	if len(d.Local) > 0 || d.LocalErr != nil {
		b.WriteString(headerPlain(strings.ToUpper(localTitle(d))) + "\n")
		b.WriteString(plainItems(d.Local, d.LocalErr))
		b.WriteString("\n")
	}
	if len(d.World) > 0 || d.WorldErr != nil {
		b.WriteString(headerPlain("WORLD") + "\n")
		b.WriteString(plainItems(d.World, d.WorldErr))
		b.WriteString("\n")
	}
	if len(d.Tech) > 0 || d.TechErr != nil {
		b.WriteString(headerPlain("TECH") + "\n")
		b.WriteString(plainItems(d.Tech, d.TechErr))
		b.WriteString("\n")
	}
	if len(d.Facts) > 0 || d.FactsErr != nil {
		b.WriteString(headerPlain(strings.ToUpper(factTitle(d))) + "\n")
		if len(d.Facts) == 0 {
			fmt.Fprintf(&b, "  unavailable: %s\n", short(d.FactsErr))
		}
		for i, f := range d.Facts {
			if i >= 2 {
				break
			}
			if f.Year > 0 {
				fmt.Fprintf(&b, "  - %d: %s\n", f.Year, oneLine(f.Text))
			} else {
				fmt.Fprintf(&b, "  - %s\n", oneLine(f.Text))
			}
		}
		b.WriteString("\n")
	}

	_, err := io.WriteString(w, strings.TrimRight(b.String(), "\n")+"\n")
	return err
}

func headerPlain(title string) string {
	return title + "\n" + strings.Repeat("-", len(title))
}

func plainItems(list []news.Item, err error) string {
	if len(list) == 0 {
		return fmt.Sprintf("  unavailable: %s\n", short(err))
	}
	var b strings.Builder
	for _, it := range list {
		tag := it.Source
		if it.Score > 0 {
			tag = fmt.Sprintf("%s, %d points", it.Source, it.Score)
		}
		if tag == "" {
			fmt.Fprintf(&b, "  - %s\n", oneLine(it.Title))
			continue
		}
		fmt.Fprintf(&b, "  - %s (%s)\n", oneLine(it.Title), tag)
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
