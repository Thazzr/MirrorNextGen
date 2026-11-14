package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/thazzr/mirrornextgen/server/internal/icons"
)

const (
	msgIconInUse   = "Das Symbol wird noch verwendet."
	msgNoFile      = "Bitte wähle eine Datei."
	maxIconNameLen = 1024
	iconCSP        = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
)

func toIcon(ic icons.Icon) Icon {
	return Icon{Id: ic.ID, Name: ic.Name, ContentType: IconContentType(ic.ContentType), Size: ic.Size}
}

// ListIcons lists the uploaded icons.
func (s *Server) ListIcons(_ context.Context, _ ListIconsRequestObject) (ListIconsResponseObject, error) {
	list := s.icons.List()
	out := make(ListIcons200JSONResponse, 0, len(list))
	for _, ic := range list {
		out = append(out, toIcon(ic))
	}
	return out, nil
}

// UploadIcon stores an uploaded SVG or PNG. The type comes from the content,
// never from the client's headers.
func (s *Server) UploadIcon(_ context.Context, request UploadIconRequestObject) (UploadIconResponseObject, error) {
	bad := func(field, msg string) (UploadIconResponseObject, error) {
		return UploadIcon400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(map[string]string{field: msg}))}, nil
	}
	var data []byte
	var fileName, name string
	haveFile := false
	for {
		part, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return bad("file", msgNoFile)
		}
		switch part.FormName() {
		case "file":
			b, err := io.ReadAll(io.LimitReader(part, icons.MaxSize+1))
			if err != nil {
				return bad("file", msgNoFile)
			}
			data, fileName, haveFile = b, part.FileName(), true
		case "name":
			b, err := io.ReadAll(io.LimitReader(part, maxIconNameLen+1))
			if err != nil || len(b) > maxIconNameLen {
				return bad("name", icons.ErrName.Error())
			}
			name = string(b)
		}
	}
	if !haveFile {
		return bad("file", msgNoFile)
	}
	ic, err := s.icons.Add(name, fileName, data)
	switch {
	case errors.Is(err, icons.ErrName):
		return bad("name", err.Error())
	case errors.Is(err, icons.ErrTooLarge), errors.Is(err, icons.ErrType), errors.Is(err, icons.ErrUnsafe):
		return bad("file", err.Error())
	case err != nil:
		return nil, err
	}
	return UploadIcon201JSONResponse(toIcon(ic)), nil
}

// DeleteIcon removes an icon that no calendar setting references.
func (s *Server) DeleteIcon(_ context.Context, request DeleteIconRequestObject) (DeleteIconResponseObject, error) {
	s.iconMu.Lock()
	defer s.iconMu.Unlock()
	if !s.icons.Exists(request.Id) {
		return DeleteIcon404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, ""))}, nil
	}
	if s.settings.Get().Calendar.UsesIcon(request.Id) {
		return DeleteIcon409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, msgIconInUse))}, nil
	}
	if _, err := s.icons.Delete(request.Id); err != nil {
		return nil, err
	}
	return DeleteIcon204Response{}, nil
}

// GetIconFile serves the bytes of an icon with headers that keep a hostile
// file from running anything.
func (s *Server) GetIconFile(_ context.Context, request GetIconFileRequestObject) (GetIconFileResponseObject, error) {
	ic, data, ok, err := s.icons.File(request.Id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return GetIconFile404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, ""))}, nil
	}
	csp, sniff, cache := iconCSP, "nosniff", "public, max-age=86400"
	headers := GetIconFile200ResponseHeaders{ContentSecurityPolicy: &csp, XContentTypeOptions: &sniff, CacheControl: &cache}
	if ic.ContentType == icons.TypePNG {
		return GetIconFile200ImagepngResponse{Body: bytes.NewReader(data), ContentLength: int64(len(data)), Headers: headers}, nil
	}
	return GetIconFile200ImagesvgXmlResponse{Body: bytes.NewReader(data), ContentLength: int64(len(data)), Headers: headers}, nil
}
