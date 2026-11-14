package api

import (
	"context"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/thazzr/mirrornextgen/server/internal/weather"
	"github.com/thazzr/mirrornextgen/server/internal/wifi"
)

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toWeatherData(d *weather.Data, showIcons bool) *WeatherData {
	out := &WeatherData{
		Location:                 d.Location,
		UpdatedAt:                d.UpdatedAt,
		TemperatureC:             d.TemperatureC,
		FeelsLikeC:               d.FeelsLikeC,
		Humidity:                 d.Humidity,
		Condition:                d.Condition,
		IconCode:                 optional(d.IconCode),
		ShowIcons:                showIcons,
		WindKph:                  d.WindKph,
		PrecipitationProbability: d.PrecipitationProbability,
		Sunrise:                  d.Sunrise,
		Sunset:                   d.Sunset,
		UvIndex:                  &d.UVIndex,
		Alert:                    optional(d.Alert),
		Hourly:                   make([]HourlyPoint, 0, len(d.Hourly)),
		Forecast:                 make([]ForecastDay, 0, len(d.Forecast)),
	}
	for _, p := range d.Hourly {
		out.Hourly = append(out.Hourly, HourlyPoint{
			Time:                     p.Time,
			TemperatureC:             p.TemperatureC,
			FeelsLikeC:               p.FeelsLikeC,
			Condition:                p.Condition,
			IconCode:                 optional(p.IconCode),
			PrecipitationProbability: p.PrecipitationProbability,
			WindKph:                  p.WindKph,
			GustKph:                  p.GustKph,
			PrecipitationMm:          p.PrecipitationMm,
		})
	}
	for _, f := range d.Forecast {
		date, err := time.Parse(time.DateOnly, f.Date)
		if err != nil {
			continue // the service formats the date itself; cannot happen
		}
		out.Forecast = append(out.Forecast, ForecastDay{
			Date:                     openapi_types.Date{Time: date},
			Condition:                f.Condition,
			IconCode:                 optional(f.IconCode),
			LowC:                     f.LowC,
			HighC:                    f.HighC,
			PrecipitationProbability: f.PrecipitationProbability,
			WindKph:                  f.WindKph,
			PrecipitationMm:          f.PrecipitationMm,
		})
	}
	return out
}

// GetDashboardWeather returns the cached weather; a failure travels in the
// body so the other dashboard areas stay visible.
func (s *Server) GetDashboardWeather(ctx context.Context, _ GetDashboardWeatherRequestObject) (GetDashboardWeatherResponseObject, error) {
	res := s.weather.Get(ctx)
	out := GetDashboardWeather200JSONResponse{Configured: res.Configured, Stale: res.Stale, Error: optional(res.Error)}
	if res.Data != nil {
		out.Data = toWeatherData(res.Data, res.ShowIcons)
	}
	return out, nil
}

// GetDashboardWifi returns the QR code of the guest Wi-Fi.
func (s *Server) GetDashboardWifi(_ context.Context, _ GetDashboardWifiRequestObject) (GetDashboardWifiResponseObject, error) {
	w := s.settings.Get().Wifi
	if w.SSID == "" {
		return GetDashboardWifi200JSONResponse{}, nil
	}
	m, err := wifi.Encode(wifi.Payload(w.SSID, w.Security, w.Password))
	if err != nil {
		return nil, fmt.Errorf("render wifi QR code: %w", err)
	}
	ssid := w.SSID
	return GetDashboardWifi200JSONResponse{Configured: true, Ssid: &ssid, Qr: &QrCode{Size: m.Size, Rows: m.Rows}}, nil
}

// GetDashboardBring returns the cached shopping list; a failure travels in the
// body so the other dashboard areas stay visible.
func (s *Server) GetDashboardBring(ctx context.Context, _ GetDashboardBringRequestObject) (GetDashboardBringResponseObject, error) {
	res := s.bring.Get(ctx)
	out := GetDashboardBring200JSONResponse{Configured: res.Configured, Stale: res.Stale, Error: optional(res.Error), Items: []BringItem{}}
	if res.Data != nil {
		for _, it := range res.Data.Items {
			out.Items = append(out.Items, BringItem{Name: it.Name, Specification: optional(it.Specification), IconUrl: it.IconURL})
		}
	}
	return out, nil
}
