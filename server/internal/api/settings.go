package api

import (
	"context"
	"net/http"

	"github.com/thazzr/mirrornextgen/server/internal/display"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

func toDisplay(d settings.DisplaySettings) DisplaySettings {
	return DisplaySettings{
		Rotation:    DisplaySettingsRotation(d.Rotation),
		Output:      d.Output,
		TestPattern: d.TestPattern,
		Schedule:    DisplaySchedule{Enabled: d.Schedule.Enabled, OffTime: d.Schedule.OffTime, OnTime: d.Schedule.OnTime},
	}
}

// GetDisplaySettings returns the stored display settings.
func (s *Server) GetDisplaySettings(_ context.Context, _ GetDisplaySettingsRequestObject) (GetDisplaySettingsResponseObject, error) {
	return GetDisplaySettings200JSONResponse(toDisplay(s.settings.Get().Display)), nil
}

// UpdateDisplaySettings validates and stores new display settings. The enum
// of the schema is not enforced while decoding (the generated server does not
// validate), so an out-of-range rotation reaches the store and yields the
// German field error.
func (s *Server) UpdateDisplaySettings(_ context.Context, request UpdateDisplaySettingsRequestObject) (UpdateDisplaySettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateDisplay(settings.DisplaySettings{
		Rotation:    int(b.Rotation),
		Output:      b.Output,
		TestPattern: b.TestPattern,
		Schedule:    display.Schedule{Enabled: b.Schedule.Enabled, OffTime: b.Schedule.OffTime, OnTime: b.Schedule.OnTime},
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		p := problem(http.StatusBadRequest, "")
		p.Errors = &errs
		return UpdateDisplaySettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	}
	return UpdateDisplaySettings200JSONResponse(toDisplay(stored)), nil
}

// GetKioskState returns the target state the kiosk agent applies.
func (s *Server) GetKioskState(_ context.Context, _ GetKioskStateRequestObject) (GetKioskStateResponseObject, error) {
	d := s.settings.Get().Display
	return GetKioskState200JSONResponse{
		Rotation:    KioskStateRotation(d.Rotation),
		Output:      d.Output,
		TestPattern: d.TestPattern,
		PowerOn:     s.display.PowerOn(),
	}, nil
}

func toDisplayStatus(st display.Status) DisplayStatus {
	out := DisplayStatus{PowerOn: st.PowerOn, Reason: DisplayStatusReason(st.Reason), Now: st.Now, TimeZone: st.TimeZone}
	if st.NextSwitchAt != nil {
		at, on := *st.NextSwitchAt, st.NextSwitchOn
		out.NextSwitchAt, out.NextSwitchOn = &at, &on
	}
	return out
}

// GetDisplayStatus tells the admin whether the monitor is on and when the
// schedule switches next.
func (s *Server) GetDisplayStatus(_ context.Context, _ GetDisplayStatusRequestObject) (GetDisplayStatusResponseObject, error) {
	return GetDisplayStatus200JSONResponse(toDisplayStatus(s.display.Status())), nil
}

// TestDisplayOff keeps the monitor off for a few seconds, whatever the schedule says.
func (s *Server) TestDisplayOff(_ context.Context, _ TestDisplayOffRequestObject) (TestDisplayOffResponseObject, error) {
	return TestDisplayOff200JSONResponse(toDisplayStatus(s.display.TestOff())), nil
}
