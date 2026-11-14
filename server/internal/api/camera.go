package api

import (
	"context"

	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

func toCameraSettings(c settings.CameraSettings) CameraSettings {
	// The stream address stays on the server; only its presence and a hint
	// of scheme and host are reported.
	return CameraSettings{
		Enabled:       c.Enabled,
		Title:         c.Title,
		HasStreamUrl:  c.StreamURL != "",
		StreamUrlHint: settings.CameraURLHint(c.StreamURL),
		Brightness:    c.Brightness,
		Contrast:      c.Contrast,
		Saturation:    c.Saturation,
		Grayscale:     c.Grayscale,
	}
}

// GetCameraSettings returns the camera settings without the stream address.
func (s *Server) GetCameraSettings(_ context.Context, _ GetCameraSettingsRequestObject) (GetCameraSettingsResponseObject, error) {
	return GetCameraSettings200JSONResponse(toCameraSettings(s.settings.Get().Camera)), nil
}

// UpdateCameraSettings stores new camera settings and has the camera service
// register (or drop) the stream.
func (s *Server) UpdateCameraSettings(_ context.Context, request UpdateCameraSettingsRequestObject) (UpdateCameraSettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateCamera(settings.CameraUpdate{
		Enabled:        b.Enabled,
		Title:          b.Title,
		StreamURL:      deref(b.StreamUrl),
		ClearStreamURL: deref(b.ClearStreamUrl),
		Brightness:     b.Brightness,
		Contrast:       b.Contrast,
		Saturation:     b.Saturation,
		Grayscale:      b.Grayscale,
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateCameraSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	if s.camera != nil {
		s.camera.Apply()
	}
	return UpdateCameraSettings200JSONResponse(toCameraSettings(stored)), nil
}

// GetDashboardCamera tells the dashboard whether and how to show the camera.
func (s *Server) GetDashboardCamera(_ context.Context, _ GetDashboardCameraRequestObject) (GetDashboardCameraResponseObject, error) {
	c := s.settings.Get().Camera
	return GetDashboardCamera200JSONResponse{
		Configured: c.Configured(),
		Title:      c.Title,
		Brightness: c.Brightness,
		Contrast:   c.Contrast,
		Saturation: c.Saturation,
		Grayscale:  c.Grayscale,
	}, nil
}
