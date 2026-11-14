package config

import (
	"strings"
	"testing"
)

const (
	owDefault    = "https://api.openweathermap.org/"
	bringDefault = "https://api.getbring.com/rest/v2/"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{"defaults", nil, Config{Addr: ":8080", DataDir: "/data", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"overrides", map[string]string{"MIRROR_ADDR": "127.0.0.1:9000", "MIRROR_DATA_DIR": "/tmp/mirror"}, Config{Addr: "127.0.0.1:9000", DataDir: "/tmp/mirror", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"blank is default", map[string]string{"MIRROR_ADDR": "", "MIRROR_DATA_DIR": ""}, Config{Addr: ":8080", DataDir: "/data", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"whitespace is default", map[string]string{"MIRROR_ADDR": "  \t", "MIRROR_DATA_DIR": " "}, Config{Addr: ":8080", DataDir: "/data", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"values are trimmed", map[string]string{"MIRROR_ADDR": " :9090 ", "MIRROR_DATA_DIR": " /x "}, Config{Addr: ":9090", DataDir: "/x", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"ipv6", map[string]string{"MIRROR_ADDR": "[::1]:8080"}, Config{Addr: "[::1]:8080", DataDir: "/data", OpenWeatherURL: owDefault, BringURL: bringDefault}},
		{"bring override", map[string]string{"MIRROR_BRING_URL": " http://127.0.0.1:9998/ "}, Config{Addr: ":8080", DataDir: "/data", OpenWeatherURL: owDefault, BringURL: "http://127.0.0.1:9998/"}},
		{"openweather override", map[string]string{"MIRROR_OPENWEATHER_URL": " http://127.0.0.1:9999/ "}, Config{Addr: ":8080", DataDir: "/data", OpenWeatherURL: "http://127.0.0.1:9999/", BringURL: bringDefault}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(env(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadInvalidAddr(t *testing.T) {
	for _, addr := range []string{"8080", "localhost", "a:b:c", "1.2.3.4"} {
		t.Run(addr, func(t *testing.T) {
			_, err := Load(env(map[string]string{"MIRROR_ADDR": addr}))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "MIRROR_ADDR") {
				t.Errorf("error %q does not name MIRROR_ADDR", err)
			}
		})
	}
}
