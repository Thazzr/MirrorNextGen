package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/thazzr/mirrornextgen/server/internal/bring"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

func toWeatherSettings(w settings.WeatherSettings) WeatherSettings {
	// The key itself stays on the server; only its presence is reported.
	return WeatherSettings{Location: w.Location, HasApiKey: w.APIKey != "", ShowIcons: w.ShowIcons, RefreshMinutes: w.RefreshMinutes}
}

func toWifiSettings(w settings.WifiSettings) WifiSettings {
	return WifiSettings{Ssid: w.SSID, Security: WifiSettingsSecurity(w.Security), HasPassword: w.Password != ""}
}

func badRequest(errs map[string]string) Problem {
	p := problem(http.StatusBadRequest, "")
	p.Errors = &errs
	return p
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// GetWeatherSettings returns the weather settings without the API key.
func (s *Server) GetWeatherSettings(_ context.Context, _ GetWeatherSettingsRequestObject) (GetWeatherSettingsResponseObject, error) {
	return GetWeatherSettings200JSONResponse(toWeatherSettings(s.settings.Get().Weather)), nil
}

// UpdateWeatherSettings stores new weather settings and drops the cached
// weather so the next dashboard poll uses them.
func (s *Server) UpdateWeatherSettings(_ context.Context, request UpdateWeatherSettingsRequestObject) (UpdateWeatherSettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateWeather(settings.WeatherUpdate{
		Location:       b.Location,
		APIKey:         deref(b.ApiKey),
		ClearAPIKey:    deref(b.ClearApiKey),
		ShowIcons:      b.ShowIcons,
		RefreshMinutes: b.RefreshMinutes,
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateWeatherSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	s.weather.Invalidate()
	return UpdateWeatherSettings200JSONResponse(toWeatherSettings(stored)), nil
}

// GetWifiSettings returns the guest Wi-Fi settings without the password.
func (s *Server) GetWifiSettings(_ context.Context, _ GetWifiSettingsRequestObject) (GetWifiSettingsResponseObject, error) {
	return GetWifiSettings200JSONResponse(toWifiSettings(s.settings.Get().Wifi)), nil
}

// UpdateWifiSettings validates and stores new guest Wi-Fi settings. The enum
// of the schema is not enforced while decoding, so an unknown security value
// reaches the store and yields the German field error.
func (s *Server) UpdateWifiSettings(_ context.Context, request UpdateWifiSettingsRequestObject) (UpdateWifiSettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateWifi(settings.WifiUpdate{
		SSID:          b.Ssid,
		Security:      string(b.Security),
		Password:      deref(b.Password),
		ClearPassword: deref(b.ClearPassword),
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateWifiSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	return UpdateWifiSettings200JSONResponse(toWifiSettings(stored)), nil
}

func toBringSettings(b settings.BringSettings) BringSettings {
	// The password itself stays on the server; only its presence is reported.
	return BringSettings{Email: b.Email, HasPassword: b.Password != "", ListUuid: b.ListUUID, MaxItems: b.MaxItems, RefreshMinutes: b.RefreshMinutes}
}

// GetBringSettings returns the Bring! settings without the password.
func (s *Server) GetBringSettings(_ context.Context, _ GetBringSettingsRequestObject) (GetBringSettingsResponseObject, error) {
	return GetBringSettings200JSONResponse(toBringSettings(s.settings.Get().Bring)), nil
}

// UpdateBringSettings stores new Bring! settings and drops the cached list so
// the next dashboard poll uses them.
func (s *Server) UpdateBringSettings(_ context.Context, request UpdateBringSettingsRequestObject) (UpdateBringSettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateBring(settings.BringUpdate{
		Email:          b.Email,
		Password:       deref(b.Password),
		ClearPassword:  deref(b.ClearPassword),
		ListUUID:       b.ListUuid,
		MaxItems:       b.MaxItems,
		RefreshMinutes: b.RefreshMinutes,
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateBringSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	s.bring.Invalidate()
	return UpdateBringSettings200JSONResponse(toBringSettings(stored)), nil
}

// GetBringLists asks Bring! for the lists of the stored account.
func (s *Server) GetBringLists(ctx context.Context, _ GetBringListsRequestObject) (GetBringListsResponseObject, error) {
	lists, err := s.bring.Lists(ctx)
	switch {
	case errors.Is(err, bring.ErrNotConfigured):
		return GetBringLists409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, "Bitte speichere zuerst E-Mail und Passwort."))}, nil
	case errors.Is(err, bring.ErrLoginFailed):
		return GetBringLists400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(problem(http.StatusBadRequest, bring.MsgLoginFailed))}, nil
	case err != nil:
		return GetBringLists502ApplicationProblemPlusJSONResponse(problem(http.StatusBadGateway, bring.MsgListsFailed)), nil
	}
	out := make(GetBringLists200JSONResponse, 0, len(lists))
	for _, l := range lists {
		out = append(out, BringList{Uuid: l.UUID, Name: l.Name})
	}
	return out, nil
}
