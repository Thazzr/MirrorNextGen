package weather

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestCapitalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"leichter Regen", "Leichter Regen"},
		{"überwiegend bewölkt", "Überwiegend bewölkt"},
		{"ärmlich", "Ärmlich"},
		{"Klarer Himmel", "Klarer Himmel"},
		{"", ""},
		{"   ", ""},
		{"x", "X"},
	}
	for _, tt := range tests {
		if got := capitalize(tt.in); got != tt.want {
			t.Errorf("capitalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestKphAndRounding(t *testing.T) {
	tests := []struct {
		ms   float64
		want int
	}{
		{0, 0}, {1, 4}, {3.2, 12}, {10, 36}, {13.9, 50}, // 50.04
		{13.8, 50}, // 49.68
		{0.5, 2},   // 1.8
	}
	for _, tt := range tests {
		if got := kph(tt.ms); got != tt.want {
			t.Errorf("kph(%v) = %d, want %d", tt.ms, got, tt.want)
		}
	}
	// Half rounds to even like .NET's Math.Round.
	for in, want := range map[float64]int{0.5: 0, 1.5: 2, 2.5: 2, 3.5: 4, -0.5: 0, -1.5: -2, 8.4: 8, 8.6: 9} {
		if got := roundInt(in); got != want {
			t.Errorf("roundInt(%v) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[float64]float64{0.25: 0.2, 0.35: 0.4, 1.84: 1.8, 12.46: 12.5, 0: 0} {
		if got := round1(in); got != want {
			t.Errorf("round1(%v) = %v, want %v", in, got, want)
		}
	}
}

func daily(loc *time.Location, y int, m time.Month, d, hour int, desc string, lo, hi float64) upstreamDaily {
	var u upstreamDaily
	u.Time = time.Date(y, m, d, hour, 0, 0, 0, loc).Unix()
	u.Temp.Min, u.Temp.Max = lo, hi
	u.Weather = []upstreamWeather{{Description: desc, Icon: "10d"}}
	u.Pop = 0.456
	u.WindSpeed = 5
	u.Rain = 1.84
	return u
}

func TestForecastDays(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 22, 30, 0, 0, loc)
	days := []upstreamDaily{
		daily(loc, 2026, 11, 13, 12, "gestern", 0, 1), // past
		daily(loc, 2026, 11, 14, 12, "heute", 0, 1),   // today is not part of it
		daily(loc, 2026, 11, 16, 12, "übermorgen", 2.5, 7.5),
		daily(loc, 2026, 11, 15, 12, "morgen", -1.5, 4.4), // unsorted input
		daily(loc, 2026, 11, 17, 12, "tag drei", 0, 1),
		daily(loc, 2026, 11, 18, 12, "tag vier", 0, 1), // beyond three days
	}
	got := forecastDays(days, now, loc)
	if len(got) != 3 {
		t.Fatalf("got %d days: %+v", len(got), got)
	}
	wantDates := []string{"2026-11-15", "2026-11-16", "2026-11-17"}
	for i, d := range got {
		if d.Date != wantDates[i] {
			t.Errorf("day %d date = %s, want %s", i, d.Date, wantDates[i])
		}
	}
	first := got[0]
	want := ForecastDay{Date: "2026-11-15", Condition: "Morgen", IconCode: "10d", LowC: -2, HighC: 4,
		PrecipitationProbability: 46, WindKph: 18, PrecipitationMm: 1.8}
	if first != want {
		t.Errorf("first = %+v, want %+v", first, want)
	}
	if got[1].LowC != 2 || got[1].HighC != 8 { // 2.5 -> 2 (even), 7.5 -> 8 (even)
		t.Errorf("banker's rounding: %+v", got[1])
	}
}

func TestForecastDaysUseLocalCalendarDay(t *testing.T) {
	loc := berlin(t)
	// 23:30 UTC on the 14th is already the 15th in Berlin (UTC+1).
	d := upstreamDaily{Time: time.Date(2026, 11, 14, 23, 30, 0, 0, time.UTC).Unix()}
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, loc)
	got := forecastDays([]upstreamDaily{d}, now, loc)
	if len(got) != 1 || got[0].Date != "2026-11-15" {
		t.Errorf("got %+v", got)
	}
	// Around the end of summer time a day is still one calendar day.
	now = time.Date(2026, 10, 24, 20, 0, 0, 0, loc)
	days := []upstreamDaily{daily(loc, 2026, 10, 25, 12, "a", 0, 1), daily(loc, 2026, 10, 26, 12, "b", 0, 1)}
	got = forecastDays(days, now, loc)
	if len(got) != 2 || got[0].Date != "2026-10-25" || got[1].Date != "2026-10-26" {
		t.Errorf("DST change: %+v", got)
	}
	// Midnight now: tomorrow is decided by the local date, not by 24 hours.
	now = time.Date(2026, 10, 25, 0, 10, 0, 0, loc)
	got = forecastDays(days, now, loc)
	if len(got) != 1 || got[0].Date != "2026-10-26" {
		t.Errorf("on the 25-hour day: %+v", got)
	}
}

func TestChartTimes(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name string
		now  time.Time
		want []string // local wall-clock, RFC3339 with offset
	}{
		{"normal day", time.Date(2026, 11, 14, 13, 5, 0, 0, loc), []string{
			"2026-11-14T00:00:00+01:00", "2026-11-14T03:00:00+01:00", "2026-11-14T06:00:00+01:00",
			"2026-11-14T09:00:00+01:00", "2026-11-14T12:00:00+01:00", "2026-11-14T15:00:00+01:00",
			"2026-11-14T18:00:00+01:00", "2026-11-14T21:00:00+01:00", "2026-11-15T00:00:00+01:00"}},
		// 2026-03-29: clocks jump from 02:00 to 03:00, so 00:00 is +01:00 and
		// the following points +02:00 (a naive "+3 hours" would give 04:00).
		{"spring forward", time.Date(2026, 3, 29, 10, 0, 0, 0, loc), []string{
			"2026-03-29T00:00:00+01:00", "2026-03-29T03:00:00+02:00", "2026-03-29T06:00:00+02:00",
			"2026-03-29T09:00:00+02:00", "2026-03-29T12:00:00+02:00", "2026-03-29T15:00:00+02:00",
			"2026-03-29T18:00:00+02:00", "2026-03-29T21:00:00+02:00", "2026-03-30T00:00:00+02:00"}},
		// 2026-10-25: clocks go back from 03:00 to 02:00; 25 hours, still nine
		// points at the local wall-clock times.
		{"fall back", time.Date(2026, 10, 25, 10, 0, 0, 0, loc), []string{
			"2026-10-25T00:00:00+02:00", "2026-10-25T03:00:00+01:00", "2026-10-25T06:00:00+01:00",
			"2026-10-25T09:00:00+01:00", "2026-10-25T12:00:00+01:00", "2026-10-25T15:00:00+01:00",
			"2026-10-25T18:00:00+01:00", "2026-10-25T21:00:00+01:00", "2026-10-26T00:00:00+01:00"}},
		// The local day, not the UTC day, decides: 23:30 UTC is already the 15th.
		{"local day differs from UTC day", time.Date(2026, 11, 14, 23, 30, 0, 0, time.UTC), []string{
			"2026-11-15T00:00:00+01:00", "2026-11-15T03:00:00+01:00", "2026-11-15T06:00:00+01:00",
			"2026-11-15T09:00:00+01:00", "2026-11-15T12:00:00+01:00", "2026-11-15T15:00:00+01:00",
			"2026-11-15T18:00:00+01:00", "2026-11-15T21:00:00+01:00", "2026-11-16T00:00:00+01:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, ts := range chartTimes(tt.now, loc) {
				got = append(got, ts.Format(time.RFC3339))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}

type fakeHistory struct {
	calls []time.Time
	// answers by instant; a missing entry means "no value".
	answers map[int64]HourlyPoint
	err     error
}

func (f *fakeHistory) fetch(_ context.Context, at time.Time) (HourlyPoint, bool, error) {
	f.calls = append(f.calls, at)
	if f.err != nil {
		return HourlyPoint{}, false, f.err
	}
	p, ok := f.answers[at.Unix()]
	return p, ok, nil
}

func TestChartPointsFillsPastPointsOncePerPoint(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 13, 5, 0, 0, loc)
	cache := newHourlyCache()
	hist := &fakeHistory{answers: map[int64]HourlyPoint{}}
	times := chartTimes(now, loc)
	for _, ts := range times[:5] { // 00:00 .. 12:00 are past
		hist.answers[ts.Unix()] = HourlyPoint{Time: ts, TemperatureC: ts.Hour(), Condition: "Klar"}
	}
	// 12:00 is in the forecast already: it must not be asked.
	cache.put(HourlyPoint{Time: times[4], TemperatureC: 99})
	// 15:00 and later come from the forecast.
	for _, ts := range times[5:] {
		cache.put(HourlyPoint{Time: ts, TemperatureC: 50 + ts.Hour()})
	}

	got, err := chartPoints(context.Background(), now, loc, cache, hist.fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 9 {
		t.Fatalf("got %d points", len(got))
	}
	if len(hist.calls) != 4 {
		t.Errorf("time machine asked %d times, want 4 (00, 03, 06, 09): %v", len(hist.calls), hist.calls)
	}
	for _, c := range hist.calls {
		if c.After(now) {
			t.Errorf("asked for a future point %v", c)
		}
	}
	if got[0].TemperatureC != 0 || got[3].TemperatureC != 9 || got[4].TemperatureC != 99 || got[5].TemperatureC != 65 {
		t.Errorf("wrong sources: %+v", got)
	}

	// A second run asks nobody again.
	hist.calls = nil
	if _, err := chartPoints(context.Background(), now, loc, cache, hist.fetch); err != nil {
		t.Fatal(err)
	}
	if len(hist.calls) != 0 {
		t.Errorf("asked again: %v", hist.calls)
	}
}

func TestChartPointsDoesNotRetryMissingPoints(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 13, 5, 0, 0, loc)
	cache := newHourlyCache()
	for _, ts := range chartTimes(now, loc)[5:] {
		cache.put(HourlyPoint{Time: ts})
	}
	hist := &fakeHistory{answers: map[int64]HourlyPoint{}} // knows nothing
	got, err := chartPoints(context.Background(), now, loc, cache, hist.fetch)
	if err != nil || len(got) != 4 {
		t.Fatalf("got %d points, err %v", len(got), err)
	}
	first := len(hist.calls)
	if first != 5 {
		t.Fatalf("asked %d times, want 5", first)
	}
	if _, err := chartPoints(context.Background(), now, loc, cache, hist.fetch); err != nil {
		t.Fatal(err)
	}
	if len(hist.calls) != first {
		t.Errorf("a missing point was asked again: %d calls", len(hist.calls))
	}
}

func TestChartPointsErrors(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 13, 5, 0, 0, loc)

	// A failing time machine only leaves the points out.
	cache := newHourlyCache()
	for _, ts := range chartTimes(now, loc)[5:] {
		cache.put(HourlyPoint{Time: ts})
	}
	hist := &fakeHistory{err: errors.New("boom")}
	got, err := chartPoints(context.Background(), now, loc, cache, hist.fetch)
	if err != nil || len(got) != 4 {
		t.Errorf("failing history: %d points, %v", len(got), err)
	}

	// Fewer than two points is an error.
	empty := newHourlyCache()
	if _, err := chartPoints(context.Background(), now, loc, empty, (&fakeHistory{}).fetch); err == nil {
		t.Error("expected an error for an empty chart")
	}

	// A cancelled context stops the refresh.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := chartPoints(ctx, now, loc, newHourlyCache(), (&fakeHistory{}).fetch); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestChartPointsOnDSTDays(t *testing.T) {
	loc := berlin(t)
	for _, tt := range []struct {
		name string
		now  time.Time
		want int
	}{
		{"spring forward", time.Date(2026, 3, 29, 13, 0, 0, 0, loc), 9},
		{"fall back", time.Date(2026, 10, 25, 13, 0, 0, 0, loc), 9},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache := newHourlyCache()
			hist := &fakeHistory{answers: map[int64]HourlyPoint{}}
			for _, ts := range chartTimes(tt.now, loc) {
				hist.answers[ts.Unix()] = HourlyPoint{Time: ts, TemperatureC: ts.Hour()}
				cache.put(HourlyPoint{Time: ts, TemperatureC: ts.Hour()})
			}
			got, err := chartPoints(context.Background(), tt.now, loc, cache, hist.fetch)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d points, want %d", len(got), tt.want)
			}
			for i, p := range got {
				if i > 0 && !p.Time.After(got[i-1].Time) {
					t.Errorf("not increasing at %d: %v", i, p.Time)
				}
				if p.Time.Minute() != 0 {
					t.Errorf("point %v is not on the hour", p.Time)
				}
			}
			if len(hist.calls) != 0 {
				t.Errorf("cached points were fetched: %v", hist.calls)
			}
		})
	}
}

func TestHourlyCachePrune(t *testing.T) {
	loc := berlin(t)
	c := newHourlyCache()
	yesterday := time.Date(2026, 11, 13, 23, 0, 0, 0, loc)
	today := time.Date(2026, 11, 14, 0, 0, 0, 0, loc)
	c.put(HourlyPoint{Time: yesterday})
	c.put(HourlyPoint{Time: today})
	c.tried[yesterday.Unix()] = true
	c.tried[today.Unix()] = true
	c.prune(time.Date(2026, 11, 14, 8, 0, 0, 0, loc), loc)
	if len(c.points) != 1 || len(c.tried) != 1 {
		t.Errorf("points %d tried %d, want 1 each", len(c.points), len(c.tried))
	}
	if _, ok := c.points[today.Unix()]; !ok {
		t.Error("today's point was pruned")
	}
}

func TestCurrentPrecipitation(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 13, 0, 0, 0, loc)
	chart := []HourlyPoint{
		{Time: time.Date(2026, 11, 14, 9, 0, 0, 0, loc), PrecipitationProbability: 10},
		{Time: time.Date(2026, 11, 14, 12, 0, 0, 0, loc), PrecipitationProbability: 30},
		{Time: time.Date(2026, 11, 14, 15, 0, 0, 0, loc), PrecipitationProbability: 70},
	}
	tests := []struct {
		name string
		rain float64
		now  time.Time
		want int
	}{
		{"rain right now", 0.2, now, 100},
		{"nearest is 12:00", 0, now, 30},
		{"nearest is 15:00", 0, time.Date(2026, 11, 14, 14, 0, 0, 0, loc), 70},
		{"tie goes to the earlier point", 0, time.Date(2026, 11, 14, 13, 30, 0, 0, loc), 30},
	}
	for _, tt := range tests {
		if got := currentPrecipitation(tt.rain, chart, tt.now); got != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, got, tt.want)
		}
	}
	if got := currentPrecipitation(0, nil, now); got != 0 {
		t.Errorf("empty chart = %d", got)
	}
}

func TestDayPeriod(t *testing.T) {
	loc := berlin(t)
	for h, want := range map[int]string{0: "Vormittag", 11: "Vormittag", 12: "Nachmittag", 17: "Nachmittag", 18: "Abend", 23: "Abend"} {
		if got := dayPeriod(time.Date(2026, 11, 14, h, 59, 0, 0, loc)); got != want {
			t.Errorf("hour %d = %s, want %s", h, got, want)
		}
	}
}

func TestWeatherKind(t *testing.T) {
	tests := map[string]string{
		"Gewitter mit Regen":    "thunderstorm",
		"Leichter Schneefall":   "snow",
		"Schneeregen":           "snow",
		"Leichter Regen":        "rain",
		"Regenschauer":          "rain",
		"Schauer":               "rain",
		"Klarer Himmel":         "clear",
		"Sonnig":                "clear",
		"Überwiegend bewölkt":   "cloudy",
		"Bedeckt":               "cloudy",
		"Ein paar Wolken":       "cloudy",
		"Nebel":                 "other",
		"":                      "other",
		"GEWITTER":              "thunderstorm",
		"Mäßiger Regen":         "rain",
		"Wolkenlos und klar":    "clear",
		"Überwiegend bedeckt":   "cloudy",
		"Starker Schneeschauer": "snow",
	}
	for in, want := range tests {
		if got := weatherKind(in); got != want {
			t.Errorf("weatherKind(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestWeatherAlert(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, loc)
	at := func(h int) time.Time { return time.Date(2026, 11, 14, h, 0, 0, 0, loc) }
	type pt = HourlyPoint
	cloudy := func(h int) pt { return pt{Time: at(h), Condition: "Überwiegend bewölkt", TemperatureC: 8} }

	tests := []struct {
		name  string
		chart []pt
		want  string
	}{
		{"nothing", []pt{cloudy(12), cloudy(15), cloudy(18)}, ""},
		{"no points", nil, ""},
		{"only past points", []pt{cloudy(6), cloudy(9)}, ""},
		{"upcoming within 20 minutes is ignored", []pt{cloudy(12), {Time: now.Add(15 * time.Minute), Condition: "Gewitter"}}, ""},
		{"upcoming just after 20 minutes counts", []pt{cloudy(12), {Time: now.Add(21 * time.Minute), Condition: "Gewitter"}}, "Gewitter ab ca. 12:21 Uhr möglich"},
		{"thunderstorm", []pt{cloudy(12), cloudy(15), {Time: at(18), Condition: "Gewitter", TemperatureC: 8}}, "Gewitter ab ca. 18:00 Uhr möglich"},
		{"thunderstorm beats rain", []pt{cloudy(12),
			{Time: at(15), Condition: "leichter Regen", PrecipitationProbability: 80},
			{Time: at(18), Condition: "Gewitter"}}, "Gewitter ab ca. 18:00 Uhr möglich"},
		{"rain starts by condition", []pt{cloudy(12), {Time: at(15), Condition: "Leichter Regen"}}, "Regen ab ca. 15:00 Uhr"},
		{"rain starts by probability", []pt{cloudy(12), {Time: at(15), Condition: "Überwiegend bewölkt", PrecipitationProbability: 50}}, "Regen ab ca. 15:00 Uhr"},
		{"probability 49 is no rain", []pt{cloudy(12), {Time: at(15), Condition: "Überwiegend bewölkt", TemperatureC: 8, PrecipitationProbability: 49}}, ""},
		{"rain ends", []pt{{Time: at(12), Condition: "Leichter Regen", PrecipitationProbability: 90},
			{Time: at(15), Condition: "Leichter Regen", PrecipitationProbability: 60},
			{Time: at(18), Condition: "Überwiegend bewölkt", PrecipitationProbability: 10}}, "Regen bis ca. 18:00 Uhr"},
		{"rain now by probability only, ends", []pt{{Time: at(12), Condition: "Bewölkt", PrecipitationProbability: 70},
			{Time: at(15), Condition: "Bewölkt", PrecipitationProbability: 20}}, "Regen bis ca. 15:00 Uhr"},
		{"rain all day", []pt{{Time: at(12), Condition: "Leichter Regen"}, {Time: at(15), Condition: "Leichter Regen"}}, ""},
		{"snow starts", []pt{cloudy(12), {Time: at(18), Condition: "Leichter Schneefall", TemperatureC: 8}}, "Schnee ab ca. 18:00 Uhr"},
		{"snow already falling", []pt{{Time: at(12), Condition: "Schneefall", TemperatureC: 8}, {Time: at(15), Condition: "Schneefall", TemperatureC: 8}}, ""},
		{"clearing up", []pt{cloudy(12), {Time: at(15), Condition: "Klarer Himmel", TemperatureC: 8}}, "Ab 15:00 sonniger"},
		{"clouds increase in the evening", []pt{{Time: at(12), Condition: "Klarer Himmel", TemperatureC: 8}, {Time: at(18), Condition: "Überwiegend bewölkt", TemperatureC: 8}}, "Bewölkung nimmt am Abend zu"},
		{"clouds increase in the afternoon", []pt{{Time: at(12), Condition: "Klarer Himmel", TemperatureC: 8}, {Time: at(15), Condition: "Bedeckt", TemperatureC: 8}}, "Bewölkung nimmt am Nachmittag zu"},
		{"change to something else says nothing", []pt{cloudy(12), {Time: at(15), Condition: "Nebel", TemperatureC: 8}}, ""},
		{"temperature drop of 5", []pt{cloudy(12), {Time: at(21), Condition: "Überwiegend bewölkt", TemperatureC: 3}}, "Temperatur fällt am Abend deutlich"},
		{"temperature drop of 4", []pt{cloudy(12), {Time: at(21), Condition: "Überwiegend bewölkt", TemperatureC: 4}}, ""},
		{"temperature drop in the afternoon", []pt{cloudy(12), {Time: at(15), Condition: "Überwiegend bewölkt", TemperatureC: -2}}, "Temperatur fällt am Nachmittag deutlich"},
		{"gusts of 50", []pt{cloudy(12), {Time: at(15), Condition: "Überwiegend bewölkt", TemperatureC: 8, GustKph: 50}}, "Starke Böen am Nachmittag"},
		{"gusts of 49", []pt{cloudy(12), {Time: at(15), Condition: "Überwiegend bewölkt", TemperatureC: 8, GustKph: 49}}, ""},
		{"temperature beats gusts", []pt{cloudy(12), {Time: at(18), Condition: "Überwiegend bewölkt", TemperatureC: 0, GustKph: 80}}, "Temperatur fällt am Abend deutlich"},
		{"current is the nearest point, not the first", []pt{
			{Time: at(0), Condition: "Leichter Regen", PrecipitationProbability: 90},
			{Time: at(12), Condition: "Überwiegend bewölkt", TemperatureC: 8},
			{Time: at(18), Condition: "Leichter Regen"}}, "Regen ab ca. 18:00 Uhr"},
		{"unsorted chart", []pt{{Time: at(18), Condition: "Gewitter"}, {Time: at(15), Condition: "Leichter Regen"}, cloudy(12)}, "Gewitter ab ca. 18:00 Uhr möglich"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weatherAlert(tt.chart, now); got != tt.want {
				t.Errorf("alert = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHourlyConversions(t *testing.T) {
	loc := berlin(t)
	gust := 14.0
	h := upstreamHourly{Time: time.Date(2026, 11, 14, 11, 0, 0, 0, time.UTC).Unix(), Temp: 8.5, FeelsLike: 6.49,
		Weather: []upstreamWeather{{Description: "leichter Regen", Icon: "10d"}}, Pop: 0.745, WindSpeed: 5, WindGust: &gust}
	h.Rain.OneHour = 0.4
	got := hourlyFromForecast(h, loc)
	want := HourlyPoint{Time: time.Date(2026, 11, 14, 12, 0, 0, 0, loc), TemperatureC: 8, FeelsLikeC: 6, Condition: "Leichter Regen",
		IconCode: "10d", PrecipitationProbability: 74, WindKph: 18, GustKph: 50, PrecipitationMm: 0.4}
	if got != want {
		t.Errorf("forecast point = %+v\nwant %+v", got, want)
	}
	h.WindGust = nil
	if got := hourlyFromForecast(h, loc); got.GustKph != 18 {
		t.Errorf("gust without value should equal wind: %d", got.GustKph)
	}

	at := time.Date(2026, 11, 14, 3, 0, 0, 0, loc)
	p := upstreamHistoryPoint{Temp: 2.4, FeelsLike: 0.5, Weather: []upstreamWeather{{Description: "klarer himmel", Icon: "01n"}}, WindSpeed: 2}
	hp := hourlyFromHistory(p, at)
	if hp.PrecipitationProbability != 0 || hp.Condition != "Klarer himmel" || hp.GustKph != 7 || !hp.Time.Equal(at) {
		t.Errorf("history point = %+v", hp)
	}
	p.Rain.OneHour = 0.1
	if hp := hourlyFromHistory(p, at); hp.PrecipitationProbability != 100 || hp.PrecipitationMm != 0.1 {
		t.Errorf("rainy history point = %+v", hp)
	}
}
