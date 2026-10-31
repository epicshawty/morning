package facts

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/epicshawty/morning/internal/fetch"
)

// coord is a WGS84 point.
type coord struct{ Lat, Lon float64 }

// maxSPARQLIDs caps one query; the endpoint degrades well past a few hundred.
const maxSPARQLIDs = 300

type sparqlResp struct {
	Results struct {
		Bindings []struct {
			Item struct {
				Value string `json:"value"`
			} `json:"item"`
			Coord struct {
				Value string `json:"value"`
			} `json:"coord"`
		} `json:"bindings"`
	} `json:"results"`
}

// coordinates resolves Wikidata QIDs to points in one SPARQL round trip.
//
// wbgetentities cannot help here: it ignores the claims filter and returns
// megabytes per batch. VALUES + wdt:P625 returns ~20 KB regardless of size.
func coordinates(ctx context.Context, qids []string) (map[string]coord, error) {
	out := map[string]coord{}
	if len(qids) == 0 {
		return out, nil
	}
	if len(qids) > maxSPARQLIDs {
		qids = qids[:maxSPARQLIDs]
	}

	var b strings.Builder
	b.WriteString("SELECT ?item ?coord WHERE { VALUES ?item {")
	for _, q := range qids {
		b.WriteString(" wd:")
		b.WriteString(q)
	}
	b.WriteString(" } ?item wdt:P625 ?coord . }")

	url := "https://query.wikidata.org/sparql?format=json&query=" + queryEscape(b.String())

	var r sparqlResp
	if err := fetch.GetJSON(ctx, url, &r); err != nil {
		return out, err
	}
	for _, row := range r.Results.Bindings {
		qid := row.Item.Value[strings.LastIndexByte(row.Item.Value, '/')+1:]
		if c, ok := parsePoint(row.Coord.Value); ok {
			out[qid] = c
		}
	}
	return out, nil
}

// parsePoint reads "Point(lon lat)" into a coord.
func parsePoint(wkt string) (coord, bool) {
	s := strings.TrimSpace(wkt)
	s = strings.TrimPrefix(s, "Point(")
	s = strings.TrimSuffix(s, ")")
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return coord{}, false
	}
	lon, err1 := strconv.ParseFloat(fields[0], 64)
	lat, err2 := strconv.ParseFloat(fields[1], 64)
	if err1 != nil || err2 != nil {
		return coord{}, false
	}
	return coord{Lat: lat, Lon: lon}, true
}

// haversine returns the great-circle distance in kilometres.
func haversine(a, b coord) float64 {
	const r = 6371.0
	la1, la2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat := la2 - la1
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(la1)*math.Cos(la2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

// queryEscape percent-encodes a SPARQL query for a URL.
func queryEscape(q string) string {
	var b strings.Builder
	for _, c := range []byte(q) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
