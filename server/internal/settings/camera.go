package settings

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Messages of the camera field errors.
const (
	msgCamTitle     = "Höchstens 60 Zeichen."
	msgCamURLShape  = "Die Adresse muss mit rtsp:// oder rtsps:// beginnen und einen Rechner nennen."
	msgCamURLLength = "Höchstens 2000 Zeichen."
	msgCamPercent   = "Erlaubt sind 0 bis 200 %."
)

const (
	// DefaultCameraTitle is the heading above the video while none is set.
	DefaultCameraTitle = "Kamera"
	maxCameraTitle     = 60
	maxCameraPercent   = 200
)

// CameraSettings configure the camera module. The stream address usually
// carries credentials, so it is a secret: stored here, never sent to a client.
type CameraSettings struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	// StreamURL is an rtsp:// or rtsps:// address; "" means none is stored.
	StreamURL  string `json:"streamUrl"`
	Brightness int    `json:"brightness"`
	Contrast   int    `json:"contrast"`
	Saturation int    `json:"saturation"`
	Grayscale  bool   `json:"grayscale"`
}

func defaultCamera() CameraSettings {
	return CameraSettings{Title: DefaultCameraTitle, Brightness: 100, Contrast: 100, Saturation: 100}
}

// Configured reports whether the camera is shown: enabled and with an address.
func (c CameraSettings) Configured() bool { return c.Enabled && c.StreamURL != "" }

// CameraURLHint shortens a stream address for display to scheme and host.
// Userinfo (credentials), port, path and query never appear in it.
func CameraURLHint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Hostname()
}

// ValidCameraURL accepts only rtsp and rtsps addresses with a host. Nothing
// else may reach the camera service: its API would run other sources (such as
// exec:) as commands.
func ValidCameraURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return false
	}
	switch u.Scheme { // url.Parse lower-cases the scheme
	case "rtsp", "rtsps":
	default:
		return false
	}
	return !strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

func (c CameraSettings) validate(errs map[string]string) {
	if utf8.RuneCountInString(c.Title) > maxCameraTitle {
		errs["camera.title"] = msgCamTitle
	}
	switch {
	case c.StreamURL == "":
	case len(c.StreamURL) > maxURLLength:
		errs["camera.streamUrl"] = msgCamURLLength
	case !ValidCameraURL(c.StreamURL):
		errs["camera.streamUrl"] = msgCamURLShape
	}
	for field, v := range map[string]int{"brightness": c.Brightness, "contrast": c.Contrast, "saturation": c.Saturation} {
		if v < 0 || v > maxCameraPercent {
			errs["camera."+field] = msgCamPercent
		}
	}
}

// CameraUpdate is a change of the camera section. An empty StreamURL keeps the
// stored address; ClearStreamURL deletes it.
type CameraUpdate struct {
	Enabled        bool
	Title          string
	StreamURL      string
	ClearStreamURL bool
	Brightness     int
	Contrast       int
	Saturation     int
	Grayscale      bool
}

func (u CameraUpdate) apply(cur CameraSettings) (CameraSettings, map[string]string) {
	next := CameraSettings{
		Enabled:    u.Enabled,
		Title:      strings.TrimSpace(u.Title),
		StreamURL:  cur.StreamURL,
		Brightness: u.Brightness,
		Contrast:   u.Contrast,
		Saturation: u.Saturation,
		Grayscale:  u.Grayscale,
	}
	raw := strings.TrimSpace(u.StreamURL)
	switch {
	case u.ClearStreamURL && raw != "":
		return cur, map[string]string{"streamUrl": msgSecretConflict}
	case u.ClearStreamURL:
		next.StreamURL = ""
	case raw != "":
		next.StreamURL = raw
	}
	return next, nil
}
