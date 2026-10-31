// Package weather talks to Open-Meteo, which needs no API key.
package weather

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/epicshawty/morning/internal/fetch"
)

// Place is a resolved location.
type Place struct {
	Name    string  `json:"name"`
	Region  string  `json:"region,omitempty"`
	Country string  `json:"country,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	TZ      string  `json:"tz,omitempty"`
}

// Label is the short human string for the location, e.g. "Boston, US".
func (p Place) Label() string {
	switch {
	case p.Name == "":
		return ""
	case p.Country != "":
		return p.Name + ", " + p.Country
	default:
		return p.Name
	}
}

// Report is one morning's weather, always stored in Celsius.
type Report struct {
	Place      Place   `json:"place"`
	TempC      float64 `json:"temp_c"`
	FeelsC     float64 `json:"feels_c"`
	HighC      float64 `json:"high_c"`
	LowC       float64 `json:"low_c"`
	Rain       int     `json:"rain_chance"` // percent
	WindKPH    float64 `json:"wind_kph"`
	Code       int     `json:"wmo_code"`
	Desc       string  `json:"desc"`
	Emoji      string  `json:"emoji"`
	Sunrise    string  `json:"sunrise,omitempty"` // HH:MM local
	Sunset     string  `json:"sunset,omitempty"`
	TimezoneAb string  `json:"tz_abbr,omitempty"`
}

// Temp renders a Celsius value in the requested unit.
func Temp(c float64, units string) string {
	if units == "f" {
		return strconv.Itoa(int(math.Round(c*9/5+32))) + "°"
	}
	return strconv.Itoa(int(math.Round(c))) + "°"
}

// Value renders a Celsius value with a unit suffix, e.g. "10°C".
func Value(c float64, units string) string {
	if units == "f" {
		return strconv.Itoa(int(math.Round(c*9/5+32))) + "°F"
	}
	return strconv.Itoa(int(math.Round(c))) + "°C"
}

type geoResp struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country_code"`
		Admin1    string  `json:"admin1"`
		Timezone  string  `json:"timezone"`
	} `json:"results"`
}

// Geocode resolves a free-text place name to coordinates.
func Geocode(ctx context.Context, name string) (Place, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Place{}, errors.New("empty place name")
	}
	u := "https://geocoding-api.open-meteo.com/v1/search?count=1&language=en&format=json&name=" +
		url.QueryEscape(name)

	var r geoResp
	if err := fetch.GetJSON(ctx, u, &r); err != nil {
		return Place{}, err
	}
	if len(r.Results) == 0 {
		return Place{}, fmt.Errorf("no place matched %q", name)
	}
	g := r.Results[0]
	return Place{
		Name:    g.Name,
		Region:  g.Admin1,
		Country: g.Country,
		Lat:     g.Latitude,
		Lon:     g.Longitude,
		TZ:      g.Timezone,
	}, nil
}

// LocateIP guesses the current place from the public IP. Best-effort only.
func LocateIP(ctx context.Context) (Place, error) {
	type ipinfo struct {
		City     string `json:"city"`
		Region   string `json:"region"`
		Country  string `json:"country"`
		Loc      string `json:"loc"`
		Timezone string `json:"timezone"`
		Bogon    bool   `json:"bogon"`
	}
	var ii ipinfo
	if err := fetch.GetJSON(ctx, "https://ipinfo.io/json", &ii); err == nil && ii.Loc != "" {
		lat, lon, ok := strings.Cut(ii.Loc, ",")
		if ok {
			la, e1 := strconv.ParseFloat(lat, 64)
			lo, e2 := strconv.ParseFloat(lon, 64)
			if e1 == nil && e2 == nil {
				return Place{
					Name:    ii.City,
					Region:  ii.Region,
					Country: ii.Country,
					Lat:     la,
					Lon:     lo,
					TZ:      ii.Timezone,
				}, nil
			}
		}
	}
	// Fallback: ip-api.com (free tier is HTTP-only).
	type ipapi struct {
		Status   string  `json:"status"`
		City     string  `json:"city"`
		Region   string  `json:"regionName"`
		CC       string  `json:"countryCode"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		Timezone string  `json:"timezone"`
	}
	var ia ipapi
	if err := fetch.GetJSON(ctx, "http://ip-api.com/json/?fields=status,city,regionName,countryCode,lat,lon,timezone", &ia); err != nil {
		return Place{}, err
	}
	if ia.Status != "success" {
		return Place{}, errors.New("ip geolocation failed")
	}
	return Place{
		Name:    ia.City,
		Region:  ia.Region,
		Country: ia.CC,
		Lat:     ia.Lat,
		Lon:     ia.Lon,
		TZ:      ia.Timezone,
	}, nil
}

type forecastResp struct {
	Current struct {
		Temp  float64 `json:"temperature_2m"`
		Feels float64 `json:"apparent_temperature"`
		Code  int     `json:"weather_code"`
		Wind  float64 `json:"wind_speed_10m"`
		IsDay int     `json:"is_day"`
	} `json:"current"`
	Daily struct {
		Code []int     `json:"weather_code"`
		Max  []float64 `json:"temperature_2m_max"`
		Min  []float64 `json:"temperature_2m_min"`
		Rain []int     `json:"precipitation_probability_max"`
		Rise []string  `json:"sunrise"`
		Set  []string  `json:"sunset"`
	} `json:"daily"`
	TimezoneAb string `json:"timezone_abbreviation"`
}

// Fetch pulls the current conditions plus today's daily summary.
func Fetch(ctx context.Context, p Place) (Report, error) {
	u := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s"+
			"&current=temperature_2m,apparent_temperature,weather_code,wind_speed_10m,is_day"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset"+
			"&timezone=auto&forecast_days=1&wind_speed_unit=kmh",
		strconv.FormatFloat(p.Lat, 'f', 4, 64),
		strconv.FormatFloat(p.Lon, 'f', 4, 64),
	)

	var f forecastResp
	if err := fetch.GetJSON(ctx, u, &f); err != nil {
		return Report{}, err
	}

	r := Report{
		Place:      p,
		TempC:      f.Current.Temp,
		FeelsC:     f.Current.Feels,
		WindKPH:    f.Current.Wind,
		Code:       f.Current.Code,
		TimezoneAb: f.TimezoneAb,
	}
	if n := len(f.Daily.Max); n > 0 {
		r.HighC = f.Daily.Max[0]
	}
	if n := len(f.Daily.Min); n > 0 {
		r.LowC = f.Daily.Min[0]
	}
	if n := len(f.Daily.Rain); n > 0 {
		r.Rain = f.Daily.Rain[0]
	}
	if n := len(f.Daily.Rise); n > 0 {
		r.Sunrise = clock(f.Daily.Rise[0])
	}
	if n := len(f.Daily.Set); n > 0 {
		r.Sunset = clock(f.Daily.Set[0])
	}
	if n := len(f.Daily.Code); n > 0 && r.Code == 0 {
		r.Code = f.Daily.Code[0]
	}
	r.Desc, r.Emoji = Describe(r.Code)
	return r, nil
}

// clock turns "2026-10-31T07:16" into "07:16".
func clock(iso string) string {
	if i := strings.IndexByte(iso, 'T'); i >= 0 && len(iso) >= i+6 {
		return iso[i+1 : i+6]
	}
	return iso
}

// Describe maps a WMO weather code to text and a glyph.
func Describe(code int) (string, string) {
	switch {
	case code == 0:
		return "Clear", "☀"
	case code == 1:
		return "Mainly clear", "🌤"
	case code == 2:
		return "Partly cloudy", "⛅"
	case code == 3:
		return "Overcast", "☁"
	case code == 45 || code == 48:
		return "Fog", "🌫"
	case code >= 51 && code <= 57:
		return "Drizzle", "🌦"
	case code >= 61 && code <= 67:
		return "Rain", "🌧"
	case code >= 71 && code <= 77:
		return "Snow", "🌨"
	case code >= 80 && code <= 82:
		return "Rain showers", "🌦"
	case code == 85 || code == 86:
		return "Snow showers", "🌨"
	case code >= 95:
		return "Thunderstorm", "⛈"
	}
	return "Unknown", "•"
}
