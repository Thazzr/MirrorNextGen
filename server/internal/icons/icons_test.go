package icons

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	png1x1 = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 20)...)
	okSVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><path d="M0 0h8v8z"/></svg>`
)

func svg(inner string) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">` + inner + `</svg>`)
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
		err  error
	}{
		{"png", png1x1, TypePNG, nil},
		{"svg", []byte(okSVG), TypeSVG, nil},
		{"svg with xml declaration and BOM", []byte("\xef\xbb\xbf<?xml version=\"1.0\"?>\n" + okSVG), TypeSVG, nil},
		{"svg with local href", svg(`<defs><path id="a" d="M0 0"/></defs><use href="#a"/><use xlink:href="#a"/>`), TypeSVG, nil},
		{"svg with embedded png", svg(`<image href="data:image/png;base64,AAAA"/>`), TypeSVG, nil},
		{"svg with embedded jpeg", svg(`<image xlink:href="data:image/jpeg;base64,AAAA"/>`), TypeSVG, nil},
		{"svg with harmless style", svg(`<style>.a{fill:red}</style><path class="a" d="M0 0"/>`), TypeSVG, nil},
		{"svg with local url()", svg(`<path style="fill:url(#g)" d="M0 0"/>`), TypeSVG, nil},
		{"text", []byte("hello"), "", ErrType},
		{"empty", nil, "", ErrType},
		{"gif", []byte("GIF89a......"), "", ErrType},
		{"html", []byte("<html><body></body></html>"), "", ErrType},
		{"broken xml", []byte("<svg><g></svg>"), "", ErrType},
		{"png magic only partly present", append([]byte("\x89PNG"), bytes.Repeat([]byte{0}, 20)...), "", ErrType},
		{"png magic with wrong line endings", append([]byte("\x89PNG\n\r\x1a\n"), bytes.Repeat([]byte{0}, 20)...), "", ErrType},
		{"png exactly MaxSize", append(append([]byte{}, pngMagic...), bytes.Repeat([]byte{0}, MaxSize-len(pngMagic))...), TypePNG, nil},
		{"svg exactly MaxSize", append([]byte(okSVG), bytes.Repeat([]byte(" "), MaxSize-len(okSVG))...), TypeSVG, nil},
		{"png MaxSize+1", append(append([]byte{}, pngMagic...), bytes.Repeat([]byte{0}, MaxSize+1-len(pngMagic))...), "", ErrTooLarge},
		{"svg MaxSize+1", append([]byte(okSVG), bytes.Repeat([]byte(" "), MaxSize+1-len(okSVG))...), "", ErrTooLarge},
		{"too large png", append(append([]byte{}, pngMagic...), bytes.Repeat([]byte{0}, MaxSize)...), "", ErrTooLarge},
		{"too large svg", []byte(okSVG + strings.Repeat(" ", MaxSize)), "", ErrTooLarge},
		{"script", svg(`<script>alert(1)</script>`), "", ErrUnsafe},
		{"script uppercase", svg(`<SCRIPT>alert(1)</SCRIPT>`), "", ErrUnsafe},
		{"foreignObject", svg(`<foreignObject><div/></foreignObject>`), "", ErrUnsafe},
		{"iframe", svg(`<iframe src="x"/>`), "", ErrUnsafe},
		{"embed", svg(`<embed src="x"/>`), "", ErrUnsafe},
		{"object", svg(`<object data="x"/>`), "", ErrUnsafe},
		{"audio", svg(`<audio src="x"/>`), "", ErrUnsafe},
		{"video", svg(`<video src="x"/>`), "", ErrUnsafe},
		{"link", svg(`<link rel="stylesheet" href="#a"/>`), "", ErrUnsafe},
		{"style import", svg(`<style>@import url(http://evil/x.css);</style>`), "", ErrUnsafe},
		{"style external url", svg(`<style>.a{background:url(http://evil/x.png)}</style>`), "", ErrUnsafe},
		{"style attribute external url", svg(`<path style="fill:url('http://evil/x')" d="M0 0"/>`), "", ErrUnsafe},
		{"onclick", svg(`<path onclick="alert(1)" d="M0 0"/>`), "", ErrUnsafe},
		{"onload on root", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`), "", ErrUnsafe},
		{"ONLOAD uppercase", svg(`<g ONLOAD="x"/>`), "", ErrUnsafe},
		{"javascript href", svg(`<a href="javascript:alert(1)"><path d="M0 0"/></a>`), "", ErrUnsafe},
		{"javascript xlink:href", svg(`<a xlink:href=" JavaScript:alert(1)"><path d="M0 0"/></a>`), "", ErrUnsafe},
		{"external href", svg(`<image href="http://evil/x.png"/>`), "", ErrUnsafe},
		{"data svg href", svg(`<image href="data:image/svg+xml;base64,AAAA"/>`), "", ErrUnsafe},
		{"animate href", svg(`<a><animate attributeName="href" values="javascript:alert(1)"/></a>`), "", ErrUnsafe},
		{"DOCTYPE", []byte(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + okSVG), "", ErrUnsafe},
		{"ENTITY", []byte(`<!DOCTYPE svg [<!ENTITY x "boom">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>`), "", ErrUnsafe},
		{"stylesheet instruction", []byte(`<?xml-stylesheet href="http://evil/x.css"?>` + okSVG), "", ErrUnsafe},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.data)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if tt.err == nil && got != tt.want {
				t.Errorf("type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectIgnoresTheFileNameAndTrustsContent(t *testing.T) {
	s, _ := Load(t.TempDir())
	ic, err := s.Add("", "logo.png", []byte(okSVG))
	if err != nil || ic.ContentType != TypeSVG {
		t.Fatalf("svg named .png: %+v, %v", ic, err)
	}
	if _, err := s.Add("", "logo.svg", []byte("<html>")); !errors.Is(err, ErrType) {
		t.Errorf("html named .svg: %v", err)
	}
}

func TestStoreAddListFileDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.List(); got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v", got)
	}
	a, err := s.Add("", "Sonne.svg", []byte(okSVG))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Add("  Haus  ", "x.png", png1x1)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(a.ID) || a.ID == b.ID {
		t.Errorf("ids %q %q", a.ID, b.ID)
	}
	if a.Name != "Sonne" || b.Name != "Haus" || a.ContentType != TypeSVG || b.ContentType != TypePNG || a.Size != len(okSVG) {
		t.Errorf("icons = %+v %+v", a, b)
	}
	for _, f := range []string{a.ID + ".svg", b.ID + ".png", "index.json"} {
		if _, err := os.Stat(filepath.Join(dir, "icons", f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// The index survives a restart.
	again, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.List(); len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("reloaded = %+v", got)
	}
	_, data, ok, err := again.File(b.ID)
	if err != nil || !ok || !bytes.Equal(data, png1x1) {
		t.Errorf("File = %v %v %v", len(data), ok, err)
	}
	if _, _, ok, _ := again.File("0000000000000000"); ok {
		t.Error("unknown id found")
	}

	found, err := again.Delete(a.ID)
	if err != nil || !found {
		t.Fatalf("delete: %v %v", found, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "icons", a.ID+".svg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still there: %v", err)
	}
	if again.Exists(a.ID) || !again.Exists(b.ID) {
		t.Error("Exists after delete")
	}
	if found, _ := again.Delete(a.ID); found {
		t.Error("second delete found it")
	}
	reloaded, _ := Load(dir)
	if len(reloaded.List()) != 1 {
		t.Errorf("index after delete = %+v", reloaded.List())
	}
}

func TestAddNameRules(t *testing.T) {
	s, _ := Load(t.TempDir())
	if _, err := s.Add(strings.Repeat("a", 61), "x.svg", []byte(okSVG)); !errors.Is(err, ErrName) {
		t.Errorf("long name: %v", err)
	}
	if _, err := s.Add("", ".svg", []byte(okSVG)); !errors.Is(err, ErrName) {
		t.Errorf("empty derived name: %v", err)
	}
	ic, err := s.Add("", `C:\icons\..\Mond.v2.svg`, []byte(okSVG))
	if err != nil || ic.Name != "Mond.v2" {
		t.Errorf("windows path name: %+v %v", ic, err)
	}
	if len(s.List()) != 1 {
		t.Error("a refused upload was stored")
	}
}

func TestMaxSizeIs256KB(t *testing.T) {
	if MaxSize != 256*1024 {
		t.Errorf("MaxSize = %d, want 262144", MaxSize)
	}
}
