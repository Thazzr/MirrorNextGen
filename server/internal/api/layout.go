package api

import (
	"context"

	"github.com/thazzr/mirrornextgen/server/internal/modules"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

func toLayout(l settings.Layout) Layout {
	l = l.Filtered(func(string) bool { return true }) // copy with no nil lists
	return Layout{TopLeft: l.TopLeft, TopRight: l.TopRight, Middle: l.Middle, BottomLeft: l.BottomLeft, BottomRight: l.BottomRight}
}

// GetLayoutSettings returns the stored layout.
func (s *Server) GetLayoutSettings(_ context.Context, _ GetLayoutSettingsRequestObject) (GetLayoutSettingsResponseObject, error) {
	return GetLayoutSettings200JSONResponse(toLayout(s.settings.Get().Layout)), nil
}

// UpdateLayoutSettings validates and stores a new layout; field errors are
// keyed by region name.
func (s *Server) UpdateLayoutSettings(_ context.Context, request UpdateLayoutSettingsRequestObject) (UpdateLayoutSettingsResponseObject, error) {
	b := request.Body
	stored, errs, err := s.settings.UpdateLayout(settings.Layout{
		TopLeft: b.TopLeft, TopRight: b.TopRight, Middle: b.Middle, BottomLeft: b.BottomLeft, BottomRight: b.BottomRight,
	})
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateLayoutSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	return UpdateLayoutSettings200JSONResponse(toLayout(stored)), nil
}

// ListModules lists the modules that exist under the current settings.
func (s *Server) ListModules(_ context.Context, _ ListModulesRequestObject) (ListModulesResponseObject, error) {
	infos := modules.Available(s.settings.Get().ModuleInput())
	out := make(ListModules200JSONResponse, 0, len(infos))
	for _, m := range infos {
		out = append(out, ModuleInfo{Key: m.Key, Name: m.Name})
	}
	return out, nil
}

// GetDashboardLayout returns the layout without keys of modules that are
// no longer available, so the dashboard never sees an unknown key.
func (s *Server) GetDashboardLayout(_ context.Context, _ GetDashboardLayoutRequestObject) (GetDashboardLayoutResponseObject, error) {
	return GetDashboardLayout200JSONResponse(dashboardLayout(s.settings.Get())), nil
}

// dashboardLayout is the stored layout reduced to available modules. The
// store already drops unavailable keys on load; this keeps the dashboard safe
// should a module vanish while the server runs.
func dashboardLayout(st settings.Settings) Layout {
	return toLayout(st.Layout.Filtered(st.ModuleAvailable))
}
