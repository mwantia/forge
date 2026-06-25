package session

// Language pairs a BCP 47 code with its full English name.
type Language struct {
	Code string
	Name string
}

// AllowedLanguages is the fixed ordered set accepted by the pipeline.
// The first entry is the default (English).
var AllowedLanguages = []Language{
	{"en", "English"},
	{"de", "German"},
	{"fr", "French"},
	{"es", "Spanish"},
	{"pt", "Portuguese"},
	{"it", "Italian"},
	{"nl", "Dutch"},
	{"pl", "Polish"},
	{"ru", "Russian"},
	{"ja", "Japanese"},
	{"zh", "Chinese"},
	{"ko", "Korean"},
	{"ar", "Arabic"},
}

// defaultLanguage is the zero-value fallback.
var defaultLanguage = AllowedLanguages[0]

// FindLanguage returns the Language for the given BCP 47 code.
// Returns (defaultLanguage, false) when code is empty or unrecognized.
func FindLanguage(code string) (Language, bool) {
	if code == "" {
		return defaultLanguage, false
	}

	for _, l := range AllowedLanguages {
		if l.Code == code {
			return l, true
		}
	}
	
	return defaultLanguage, false
}

// IsAllowedLanguage reports whether code is in the allowed set.
func IsAllowedLanguage(code string) bool {
	_, ok := FindLanguage(code)
	return ok
}
