package lang

import "testing"

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    Lang
	}{
		{"empty headers", []string{"", ""}, Default},
		{"strict ru", []string{"ru"}, RU},
		{"strict en", []string{"en"}, EN},
		{"ru-RU subtag", []string{"ru-RU"}, RU},
		{"en-US subtag", []string{"en-US"}, EN},
		{"unsupported defaults", []string{"de"}, Default},
		{"q-factor prefers ru", []string{"en;q=0.5,ru;q=0.9"}, RU},
		{"q-factor prefers en", []string{"de,en;q=0.9,ru;q=0.5"}, EN},
		{"x-accept-language wins over accept-language", []string{"en", "ru"}, EN},
		{"fallback to second header", []string{"", "en-US,en;q=0.9"}, EN},
		{"invalid header skipped, fallback used", []string{"!!!invalid", "ru"}, RU},
		{"unsupported in primary, fallback to second", []string{"de", "ru"}, RU},
		{"all unsupported defaults", []string{"de", "fr"}, Default},
		{"complex real-world chrome ru", []string{"ru,en-US;q=0.9,en;q=0.8,de;q=0.7"}, RU},
		{"complex real-world chrome en", []string{"en-US,en;q=0.9,de;q=0.8"}, EN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAcceptLanguage(tt.headers...)
			if got != tt.want {
				t.Errorf("ParseAcceptLanguage(%v) = %q; want %q", tt.headers, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Lang
	}{
		{"ru", RU},
		{"RU", RU},
		{" en ", EN},
		{"de", Default},
		{"", Default},
		{"ru-RU", Default},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Parse(tt.in); got != tt.want {
				t.Errorf("Parse(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}
