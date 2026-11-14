package icons

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

// Content types of the accepted formats.
const (
	TypeSVG = "image/svg+xml"
	TypePNG = "image/png"
)

// Upload errors; the messages are shown to the admin.
var (
	ErrTooLarge = errors.New("Höchstens 256 KB.")
	ErrType     = errors.New("Erlaubt sind SVG und PNG.")
	ErrUnsafe   = errors.New("Die SVG-Datei enthält nicht erlaubte Inhalte.")
	ErrName     = errors.New("Der Name muss 1 bis 60 Zeichen haben.")
)

// MaxSize is the largest accepted upload in bytes.
const MaxSize = 256 * 1024

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// forbiddenElements can run code or load active content.
var forbiddenElements = map[string]bool{
	"script": true, "foreignobject": true, "iframe": true, "embed": true,
	"object": true, "audio": true, "video": true, "link": true,
}

// externalCSS matches an @import or a url() that does not point at a local
// fragment.
var externalCSS = regexp.MustCompile(`(?i)@import|url\(\s*['"]?\s*[^#'"\s)]`)

// Detect returns the content type of data, judged by content only, and
// rejects what must not be stored. A malicious SVG is ErrUnsafe.
func Detect(data []byte) (string, error) {
	if len(data) > MaxSize {
		return "", ErrTooLarge
	}
	if bytes.HasPrefix(data, pngMagic) {
		return TypePNG, nil
	}
	trimmed := bytes.TrimLeft(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("<")) {
		return "", ErrType
	}
	return TypeSVG, checkSVG(data)
}

// checkSVG parses the XML and rejects active content. It reports ErrType when
// the data is not an SVG document at all.
func checkSVG(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	depth := 0
	sawRoot := false
	inStyle := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ErrType
		}
		switch t := tok.(type) {
		case xml.Directive:
			// DOCTYPE and ENTITY declarations enable entity attacks.
			return ErrUnsafe
		case xml.ProcInst:
			if t.Target != "xml" {
				return ErrUnsafe
			}
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if depth == 0 {
				if name != "svg" {
					return ErrType
				}
				sawRoot = true
			}
			depth++
			if forbiddenElements[name] {
				return ErrUnsafe
			}
			inStyle = name == "style"
			if !attributesSafe(t.Attr) {
				return ErrUnsafe
			}
		case xml.EndElement:
			depth--
			inStyle = false
		case xml.CharData:
			if inStyle && externalCSS.Match(t) {
				return ErrUnsafe
			}
		}
	}
	if !sawRoot {
		return ErrType
	}
	return nil
}

func attributesSafe(attrs []xml.Attr) bool {
	animatesLink := false
	for _, a := range attrs {
		local := strings.ToLower(a.Name.Local)
		val := strings.ToLower(strings.TrimSpace(a.Value))
		switch {
		case strings.HasPrefix(local, "on"):
			return false
		case local == "href":
			if !strings.HasPrefix(val, "#") &&
				!strings.HasPrefix(val, "data:image/png") &&
				!strings.HasPrefix(val, "data:image/jpeg") {
				return false
			}
		case local == "style":
			if externalCSS.MatchString(a.Value) {
				return false
			}
		case local == "attributename":
			animatesLink = val == "href" || strings.HasPrefix(val, "on")
		}
	}
	return !animatesLink
}
