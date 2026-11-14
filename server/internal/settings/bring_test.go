package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBringDefaults(t *testing.T) {
	want := BringSettings{MaxItems: 18, RefreshMinutes: 5}
	if got := Defaults().Bring; got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}
	if errs := Defaults().Validate(); errs != nil {
		t.Errorf("defaults invalid: %v", errs)
	}
}

func TestValidateBring(t *testing.T) {
	tests := []struct {
		name  string
		b     BringSettings
		field string
		msg   string
	}{
		{"defaults", Defaults().Bring, "", ""},
		{"max items 1", BringSettings{MaxItems: 1, RefreshMinutes: 5}, "", ""},
		{"max items 18", BringSettings{MaxItems: 18, RefreshMinutes: 5}, "", ""},
		{"max items 0", BringSettings{MaxItems: 0, RefreshMinutes: 5}, "bring.maxItems", "Erlaubt sind 1 bis 18 Einträge."},
		{"max items 19", BringSettings{MaxItems: 19, RefreshMinutes: 5}, "bring.maxItems", "Erlaubt sind 1 bis 18 Einträge."},
		{"refresh 1", BringSettings{MaxItems: 18, RefreshMinutes: 1}, "", ""},
		{"refresh 1440", BringSettings{MaxItems: 18, RefreshMinutes: 1440}, "", ""},
		{"refresh 0", BringSettings{MaxItems: 18, RefreshMinutes: 0}, "bring.refreshMinutes", "Erlaubt sind 1 bis 1440 Minuten."},
		{"refresh 1441", BringSettings{MaxItems: 18, RefreshMinutes: 1441}, "bring.refreshMinutes", "Erlaubt sind 1 bis 1440 Minuten."},
		{"email 200", BringSettings{MaxItems: 18, RefreshMinutes: 5, Email: strings.Repeat("ä", 200)}, "", ""},
		{"email 201", BringSettings{MaxItems: 18, RefreshMinutes: 5, Email: strings.Repeat("a", 201)}, "bring.email", "Höchstens 200 Zeichen."},
		{"password 200", BringSettings{MaxItems: 18, RefreshMinutes: 5, Password: strings.Repeat("p", 200)}, "", ""},
		{"password 201", BringSettings{MaxItems: 18, RefreshMinutes: 5, Password: strings.Repeat("p", 201)}, "bring.password", "Höchstens 200 Zeichen."},
		{"list 100", BringSettings{MaxItems: 18, RefreshMinutes: 5, ListUUID: strings.Repeat("l", 100)}, "", ""},
		{"list 101", BringSettings{MaxItems: 18, RefreshMinutes: 5, ListUUID: strings.Repeat("l", 101)}, "bring.listUuid", "Höchstens 100 Zeichen."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := Defaults()
			st.Bring = tt.b
			checkField(t, st.Validate(), tt.field, tt.msg)
		})
	}
}

func TestBringSecretSemanticsAndPersistence(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	base := BringUpdate{Email: " a@b.de ", Password: " pw 1 ", ListUUID: " L1 ", MaxItems: 12, RefreshMinutes: 10}
	got, errs, err := s.UpdateBring(base)
	if err != nil || errs != nil {
		t.Fatal(errs, err)
	}
	// Email and list are trimmed; the password is kept exactly.
	if got.Email != "a@b.de" || got.ListUUID != "L1" || got.Password != " pw 1 " {
		t.Fatalf("trim: %+v", got)
	}

	// An empty password keeps the stored one.
	keep := base
	keep.Password, keep.MaxItems = "", 17
	if got, _, _ = s.UpdateBring(keep); got.Password != " pw 1 " || got.MaxItems != 17 {
		t.Errorf("keep: %+v", got)
	}
	// It survives a reload (the secret is persisted in the file).
	s2, _ := Load(dir, quiet)
	if s2.Get().Bring.Password != " pw 1 " {
		t.Errorf("reloaded: %+v", s2.Get().Bring)
	}
	// A new password replaces it.
	repl := base
	repl.Password = "pw2"
	if got, _, _ = s.UpdateBring(repl); got.Password != "pw2" {
		t.Errorf("replace: %+v", got)
	}
	// A new password together with clear is a field error and changes nothing.
	both := base
	both.Password, both.ClearPassword = "pw3", true
	_, errs, _ = s.UpdateBring(both)
	if errs["password"] == "" {
		t.Errorf("conflict not reported: %v", errs)
	}
	if s.Get().Bring.Password != "pw2" {
		t.Error("conflict changed the password")
	}
	// Clear deletes it.
	clr := base
	clr.Password, clr.ClearPassword = "", true
	if got, _, _ = s.UpdateBring(clr); got.Password != "" {
		t.Errorf("clear: %+v", got)
	}
}

func TestBringFieldErrorsCarryNoSectionPrefixAndStoreNothing(t *testing.T) {
	s, _ := Load(t.TempDir(), quiet)
	_, errs, _ := s.UpdateBring(BringUpdate{MaxItems: 0, RefreshMinutes: 5000})
	if len(errs) != 2 || errs["maxItems"] == "" || errs["refreshMinutes"] == "" {
		t.Errorf("errs = %v", errs)
	}
	if s.Get().Bring != Defaults().Bring {
		t.Errorf("rejected update changed the state: %+v", s.Get().Bring)
	}
}

func TestBringSectionIsPartOfEqualAndFile(t *testing.T) {
	a, b := Defaults(), Defaults()
	b.Bring.Email = "x@y.de"
	if a.Equal(b) {
		t.Error("Equal ignores the Bring! section")
	}
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	if _, errs, err := s.UpdateBring(BringUpdate{Email: "x@y.de", Password: "pw", MaxItems: 18, RefreshMinutes: 5}); err != nil || errs != nil {
		t.Fatal(errs, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	for _, want := range []string{`"email": "x@y.de"`, `"password": "pw"`, `"listUuid": ""`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("%s missing in %s", want, data)
		}
	}
}

// A fresh install gets Bring! in the bottom left; a layout that is stored
// stays exactly as the user left it, even without the module.
func TestDefaultLayoutHasBringButStoredLayoutIsUntouched(t *testing.T) {
	if got := DefaultLayout().BottomLeft; !reflect.DeepEqual(got, []string{"bring"}) {
		t.Errorf("default bottomLeft = %v", got)
	}
	dir := t.TempDir()
	content := `{"schemaVersion":1,"layout":{"topLeft":["clock"],"topRight":["weather"],"middle":[],"bottomLeft":[],"bottomRight":["wifi"]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := Load(dir, quiet)
	if got := s.Get().Layout.BottomLeft; len(got) != 0 {
		t.Errorf("stored layout was changed: bottomLeft = %v", got)
	}
	if got := s.Get().Bring; got != Defaults().Bring {
		t.Errorf("bring defaults missing: %+v", got)
	}
}
