package lang

import "golang.org/x/text/language"

var (
	tags    = buildTags()
	matcher = language.NewMatcher(tags)
)

func buildTags() []language.Tag {
	out := make([]language.Tag, 0, len(Supported))
	for _, l := range Supported {
		out = append(out, language.MustParse(string(l)))
	}
	return out
}

func ParseAcceptLanguage(headers ...string) Lang {
	for _, h := range headers {
		if h == "" {
			continue
		}
		userTags, _, err := language.ParseAcceptLanguage(h)
		if err != nil || len(userTags) == 0 {
			continue
		}
		_, idx, conf := matcher.Match(userTags...)
		if conf == language.No {
			continue
		}
		return Supported[idx]
	}
	return Default
}
