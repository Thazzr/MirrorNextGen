package weather

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// HourlyPoint is one point of the day chart.
type HourlyPoint struct {
	Time                     time.Time
	TemperatureC             int
	FeelsLikeC               int
	Condition                string
	IconCode                 string
	PrecipitationProbability int
	WindKph                  int
	GustKph                  int
	PrecipitationMm          float64
}

// ForecastDay is the forecast of one local day.
type ForecastDay struct {
	// Date is the local calendar day as yyyy-MM-dd.
	Date                     string
	Condition                string
	IconCode                 string
	LowC                     int
	HighC                    int
	PrecipitationProbability int
	WindKph                  int
	PrecipitationMm          float64
}

// roundInt rounds half to even, like .NET's Math.Round, so the numbers equal
// the ones SmartMirror2 showed.
func roundInt(f float64) int { return int(math.RoundToEven(f)) }

// kph converts metres per second to whole kilometres per hour.
func kph(metresPerSecond float64) int { return roundInt(metresPerSecond * 3.6) }

// round1 rounds to one decimal place (half to even).
func round1(f float64) float64 { return math.RoundToEven(f*10) / 10 }

// capitalize upper-cases the first rune. Go's unicode tables give the same
// result as the German culture for the letters OpenWeather's German texts
// start with (including ä, ö, ü).
func capitalize(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// dayKey identifies a local calendar day.
func dayKey(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// forecastDays picks the first three local days from tomorrow on. Days are
// decided by their local calendar date, so a 23- or 25-hour day is one day.
func forecastDays(daily []upstreamDaily, now time.Time, loc *time.Location) []ForecastDay {
	tomorrow := dayKey(now.In(loc).AddDate(0, 0, 1))
	type dated struct {
		key  int
		date time.Time
		item upstreamDaily
	}
	var days []dated
	for _, d := range daily {
		local := time.Unix(d.Time, 0).In(loc)
		if k := dayKey(local); k >= tomorrow {
			days = append(days, dated{k, local, d})
		}
	}
	slices.SortStableFunc(days, func(a, b dated) int { return a.key - b.key })
	if len(days) > 3 {
		days = days[:3]
	}
	out := make([]ForecastDay, 0, len(days))
	for _, d := range days {
		w := firstWeather(d.item.Weather)
		out = append(out, ForecastDay{
			Date:                     d.date.Format("2006-01-02"),
			Condition:                capitalize(w.Description),
			IconCode:                 w.Icon,
			LowC:                     roundInt(d.item.Temp.Min),
			HighC:                    roundInt(d.item.Temp.Max),
			PrecipitationProbability: roundInt(d.item.Pop * 100),
			WindKph:                  kph(d.item.WindSpeed),
			PrecipitationMm:          round1(d.item.Rain),
		})
	}
	return out
}

func firstWeather(w []upstreamWeather) upstreamWeather {
	if len(w) == 0 {
		return upstreamWeather{}
	}
	return w[0]
}

// hourlyFromForecast converts an entry of the One Call hourly forecast.
func hourlyFromForecast(h upstreamHourly, loc *time.Location) HourlyPoint {
	w := firstWeather(h.Weather)
	gust := h.WindSpeed
	if h.WindGust != nil {
		gust = *h.WindGust
	}
	return HourlyPoint{
		Time:                     time.Unix(h.Time, 0).In(loc),
		TemperatureC:             roundInt(h.Temp),
		FeelsLikeC:               roundInt(h.FeelsLike),
		Condition:                capitalize(w.Description),
		IconCode:                 w.Icon,
		PrecipitationProbability: roundInt(h.Pop * 100),
		WindKph:                  kph(h.WindSpeed),
		GustKph:                  kph(gust),
		PrecipitationMm:          h.Rain.OneHour,
	}
}

// hourlyFromHistory converts a time machine entry for the instant at. The
// time machine knows no probability: rain in the last hour counts as 100.
func hourlyFromHistory(p upstreamHistoryPoint, at time.Time) HourlyPoint {
	w := firstWeather(p.Weather)
	gust := p.WindSpeed
	if p.WindGust != nil {
		gust = *p.WindGust
	}
	pop := 0
	if p.Rain.OneHour > 0 {
		pop = 100
	}
	return HourlyPoint{
		Time:                     at,
		TemperatureC:             roundInt(p.Temp),
		FeelsLikeC:               roundInt(p.FeelsLike),
		Condition:                capitalize(w.Description),
		IconCode:                 w.Icon,
		PrecipitationProbability: pop,
		WindKph:                  kph(p.WindSpeed),
		GustKph:                  kph(gust),
		PrecipitationMm:          p.Rain.OneHour,
	}
}

// chartTimes returns the instants of the day chart: today 00:00, 03:00, ...,
// 21:00 and tomorrow 00:00, built from the local calendar day (never by
// adding 3 hours to an instant, which drifts on the days the clocks change).
func chartTimes(now time.Time, loc *time.Location) []time.Time {
	y, m, d := now.In(loc).Date()
	out := make([]time.Time, 0, 9)
	for h := 0; h <= 24; h += 3 {
		out = append(out, time.Date(y, m, d, h, 0, 0, 0, loc))
	}
	return out
}

// hourlyCache remembers hourly points by instant, so points of the past
// (no longer in the forecast) survive until the day is over.
type hourlyCache struct {
	points map[int64]HourlyPoint
	// tried holds the past points the time machine was asked for, so each
	// is requested at most once.
	tried map[int64]bool
}

func newHourlyCache() *hourlyCache {
	return &hourlyCache{points: map[int64]HourlyPoint{}, tried: map[int64]bool{}}
}

func (c *hourlyCache) put(p HourlyPoint) { c.points[p.Time.Unix()] = p }

// prune drops everything before the start of the local day of now.
func (c *hourlyCache) prune(now time.Time, loc *time.Location) {
	y, m, d := now.In(loc).Date()
	cut := time.Date(y, m, d, 0, 0, 0, 0, loc).Unix()
	for k := range c.points {
		if k < cut {
			delete(c.points, k)
		}
	}
	for k := range c.tried {
		if k < cut {
			delete(c.tried, k)
		}
	}
}

// historyFetcher returns the weather at the past instant at; ok is false when
// the service has no value for it.
type historyFetcher func(ctx context.Context, at time.Time) (p HourlyPoint, ok bool, err error)

// chartPoints returns the day chart from the cache. Past points that are not
// cached are asked from fetch, once per point; a failing fetch leaves the
// point out. Fewer than two points is an error (no chart can be drawn).
func chartPoints(ctx context.Context, now time.Time, loc *time.Location, cache *hourlyCache, fetch historyFetcher) ([]HourlyPoint, error) {
	var out []HourlyPoint
	for _, t := range chartTimes(now, loc) {
		key := t.Unix()
		if _, ok := cache.points[key]; !ok && !t.After(now) && !cache.tried[key] {
			cache.tried[key] = true
			p, ok, err := fetch(ctx, t)
			if cerr := ctx.Err(); cerr != nil {
				return nil, fmt.Errorf("fetch past point: %w", cerr)
			}
			if err == nil && ok {
				cache.put(p)
			}
		}
		if p, ok := cache.points[key]; ok {
			// Entries from the forecast carry the instant of the hour; show
			// the chart point's own local time.
			p.Time = t
			out = append(out, p)
		}
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("only %d hourly points available", len(out))
	}
	return out, nil
}

// nearest returns the point closest to now (the first one on a tie).
func nearest(points []HourlyPoint, now time.Time) (HourlyPoint, bool) {
	if len(points) == 0 {
		return HourlyPoint{}, false
	}
	best := points[0]
	bestDiff := absDuration(best.Time.Sub(now))
	for _, p := range points[1:] {
		if d := absDuration(p.Time.Sub(now)); d < bestDiff {
			best, bestDiff = p, d
		}
	}
	return best, true
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// currentPrecipitation is 100 while it rains right now, else the probability
// of the chart point nearest to now.
func currentPrecipitation(rainLastHour float64, chart []HourlyPoint, now time.Time) int {
	if rainLastHour > 0 {
		return 100
	}
	if p, ok := nearest(chart, now); ok {
		return p.PrecipitationProbability
	}
	return 0
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), sub)
}

func isRain(condition string) bool {
	return containsFold(condition, "regen") || containsFold(condition, "schauer")
}

func isSnow(condition string) bool { return containsFold(condition, "schnee") }

// weatherKind groups German condition texts the way the alert compares them.
func weatherKind(condition string) string {
	switch {
	case containsFold(condition, "gewitter"):
		return "thunderstorm"
	case isSnow(condition):
		return "snow"
	case isRain(condition):
		return "rain"
	case containsFold(condition, "klar") || containsFold(condition, "sonn"):
		return "clear"
	case containsFold(condition, "bewölkt") || containsFold(condition, "bedeckt") || containsFold(condition, "wolke"):
		return "cloudy"
	}
	return "other"
}

func dayPeriod(t time.Time) string {
	switch h := t.Hour(); {
	case h < 12:
		return "Vormittag"
	case h < 18:
		return "Nachmittag"
	}
	return "Abend"
}

// weatherAlert is the short hint about what changes within the day; "" means
// nothing noteworthy. Thresholds and wording follow SmartMirror2.
func weatherAlert(chart []HourlyPoint, now time.Time) string {
	current, ok := nearest(chart, now)
	if !ok {
		return ""
	}
	var upcoming []HourlyPoint
	for _, p := range chart {
		if p.Time.After(now.Add(20 * time.Minute)) {
			upcoming = append(upcoming, p)
		}
	}
	if len(upcoming) == 0 {
		return ""
	}
	slices.SortStableFunc(upcoming, func(a, b HourlyPoint) int { return a.Time.Compare(b.Time) })
	first := func(match func(HourlyPoint) bool) (HourlyPoint, bool) {
		for _, p := range upcoming {
			if match(p) {
				return p, true
			}
		}
		return HourlyPoint{}, false
	}
	at := func(p HourlyPoint) string { return p.Time.Format("15:04") }

	if p, ok := first(func(p HourlyPoint) bool { return containsFold(p.Condition, "gewitter") }); ok {
		return "Gewitter ab ca. " + at(p) + " Uhr möglich"
	}

	rainy := func(p HourlyPoint) bool { return p.PrecipitationProbability >= 50 || isRain(p.Condition) }
	rainingNow := rainy(current)
	if p, ok := first(rainy); ok && !rainingNow {
		return "Regen ab ca. " + at(p) + " Uhr"
	}
	if rainingNow {
		if p, ok := first(func(p HourlyPoint) bool { return !rainy(p) }); ok {
			return "Regen bis ca. " + at(p) + " Uhr"
		}
	}

	if p, ok := first(func(p HourlyPoint) bool { return isSnow(p.Condition) }); ok && !isSnow(current.Condition) {
		return "Schnee ab ca. " + at(p) + " Uhr"
	}

	currentKind := weatherKind(current.Condition)
	if p, ok := first(func(p HourlyPoint) bool { return weatherKind(p.Condition) != currentKind }); ok {
		switch next := weatherKind(p.Condition); {
		case next == "clear":
			return "Ab " + at(p) + " sonniger"
		case next == "cloudy" && currentKind != "cloudy":
			return "Bewölkung nimmt am " + dayPeriod(p.Time) + " zu"
		case next == "rain":
			return "Regen ab ca. " + at(p) + " Uhr"
		case next == "snow":
			return "Schnee ab ca. " + at(p) + " Uhr"
		}
	}

	if p, ok := first(func(p HourlyPoint) bool { return current.TemperatureC-p.TemperatureC >= 5 }); ok {
		return "Temperatur fällt am " + dayPeriod(p.Time) + " deutlich"
	}
	if p, ok := first(func(p HourlyPoint) bool { return p.GustKph >= 50 }); ok {
		return "Starke Böen am " + dayPeriod(p.Time)
	}
	return ""
}
