package kiosk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	xcursorVersion   = 0x00010000
	xcursorImageType = 0xfffd0002
	xcursorNominal   = 24
)

// invisibleCursor is an Xcursor file with one fully transparent 1x1 image.
func invisibleCursor() []byte {
	var b bytes.Buffer
	w := func(vs ...uint32) {
		for _, v := range vs {
			_ = binary.Write(&b, binary.LittleEndian, v)
		}
	}
	const imagePos = 16 + 12             // file header + one TOC entry
	w(0x72756358, 16, xcursorVersion, 1) // "Xcur" read as little-endian uint32
	w(xcursorImageType, xcursorNominal, imagePos)
	w(36, xcursorImageType, xcursorNominal, 1, 1, 1, 0, 0, 0) // header, type, subtype, version, w, h, xhot, yhot, delay
	w(0x00000000)                                             // the one ARGB pixel
	return b.Bytes()
}

// WriteInvisibleCursorTheme writes an Xcursor theme named "default" under dir
// whose pointer images are fully transparent. Cage draws its own cursor until
// the pointer reaches the browser; with this theme that cursor is invisible.
// Point XCURSOR_PATH at dir.
func WriteInvisibleCursorTheme(dir string) error {
	cursors := filepath.Join(dir, "default", "cursors")
	if err := os.MkdirAll(cursors, 0o755); err != nil {
		return fmt.Errorf("create cursor theme dir: %w", err)
	}
	data := invisibleCursor()
	// Cage asks for "default"; older code paths use "left_ptr".
	for _, name := range []string{"default", "left_ptr"} {
		if err := os.WriteFile(filepath.Join(cursors, name), data, 0o644); err != nil {
			return fmt.Errorf("write cursor %s: %w", name, err)
		}
	}
	index := filepath.Join(dir, "default", "index.theme")
	if err := os.WriteFile(index, []byte("[Icon Theme]\nName=default\n"), 0o644); err != nil {
		return fmt.Errorf("write index.theme: %w", err)
	}
	return nil
}
