package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/logbook"
	"github.com/thazzr/mirrornextgen/server/internal/update"
)

const (
	msgUpdateBusy        = "Ein Update läuft bereits."
	msgUpdateUnavailable = "Updates per Knopf sind nicht eingerichtet."
	msgUpdateNotWritable = "Der Spiegel kann nicht in das Steuerverzeichnis schreiben. Richte den Update-Dienst ein (deploy/INSTALL.md, Abschnitt „Updates per Knopf“)."
)

func logLevel(l LogbookEntryLevel) slog.Level {
	switch l {
	case LogbookEntryLevelDebug:
		return slog.LevelDebug
	case LogbookEntryLevelWarn:
		return slog.LevelWarn
	case LogbookEntryLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// apiLevel maps a slog level to the contract's four levels (anything between
// is rounded down).
func apiLevel(l slog.Level) LogbookEntryLevel {
	switch {
	case l >= slog.LevelError:
		return LogbookEntryLevelError
	case l >= slog.LevelWarn:
		return LogbookEntryLevelWarn
	case l >= slog.LevelInfo:
		return LogbookEntryLevelInfo
	default:
		return LogbookEntryLevelDebug
	}
}

// GetLogbook returns the recent log records, newest first.
func (s *Server) GetLogbook(_ context.Context, request GetLogbookRequestObject) (GetLogbookResponseObject, error) {
	minLevel := LogbookEntryLevelInfo
	if p := request.Params.MinLevel; p != nil {
		minLevel = LogbookEntryLevel(*p)
		// The generated router does not check enum values.
		if !minLevel.Valid() {
			return GetLogbook400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(map[string]string{"minLevel": "Unbekannte Stufe."}))}, nil
		}
	}
	out := Logbook{Entries: []LogbookEntry{}, Capacity: logbook.Capacity}
	if s.logbook == nil {
		return GetLogbook200JSONResponse(out), nil
	}
	for _, e := range s.logbook.Entries(logLevel(minLevel)) {
		attrs := make([]LogbookAttr, 0, len(e.Attrs))
		for _, a := range e.Attrs {
			attrs = append(attrs, LogbookAttr{Key: a.Key, Value: a.Value})
		}
		out.Entries = append(out.Entries, LogbookEntry{Time: e.Time, Level: apiLevel(e.Level), Message: e.Message, Attrs: attrs})
	}
	return GetLogbook200JSONResponse(out), nil
}

func toUpdateStatus(st update.Status) UpdateStatus {
	u := UpdateStatus{
		Available:  st.Available,
		Pending:    st.Pending,
		Stalled:    st.Stalled,
		State:      UpdateStatusState(st.State),
		StartedAt:  st.StartedAt,
		FinishedAt: st.FinishedAt,
	}
	if st.Message != "" {
		m := st.Message
		u.Message = &m
	}
	return u
}

// GetSystemUpdate reports the state of the update button.
func (s *Server) GetSystemUpdate(_ context.Context, _ GetSystemUpdateRequestObject) (GetSystemUpdateResponseObject, error) {
	return GetSystemUpdate200JSONResponse(toUpdateStatus(s.update.Status(s.now()))), nil
}

// RequestSystemUpdate asks the host's updater to run.
func (s *Server) RequestSystemUpdate(_ context.Context, _ RequestSystemUpdateRequestObject) (RequestSystemUpdateResponseObject, error) {
	now := s.now()
	st, err := s.update.Request(now)
	switch {
	case errors.Is(err, update.ErrUnavailable):
		return RequestSystemUpdate503ApplicationProblemPlusJSONResponse(problem(http.StatusServiceUnavailable, msgUpdateUnavailable)), nil
	case errors.Is(err, update.ErrBusy):
		return RequestSystemUpdate409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, msgUpdateBusy))}, nil
	case errors.Is(err, update.ErrNotWritable):
		// The error names the control directory, which is not a secret.
		s.logger.Warn("update request could not be written", "error", err)
		return RequestSystemUpdate503ApplicationProblemPlusJSONResponse(problem(http.StatusServiceUnavailable, msgUpdateNotWritable)), nil
	case err != nil:
		return nil, err
	}
	s.logger.Info("update requested", "at", now.Format(time.RFC3339))
	return RequestSystemUpdate202JSONResponse(toUpdateStatus(st)), nil
}
