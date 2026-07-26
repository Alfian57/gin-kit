package cli

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// splitWords segments an identifier on dashes, underscores, spaces, and
// case boundaries, keeping acronym runs together (APIKey -> API, Key).
func splitWords(name string) []string {
	var words []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}
	runes := []rune(name)
	for index, r := range runes {
		switch {
		case r == '-' || r == '_' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			if len(current) > 0 {
				previous := current[len(current)-1]
				if unicode.IsLower(previous) || unicode.IsDigit(previous) {
					flush()
				} else if unicode.IsUpper(previous) && index+1 < len(runes) && unicode.IsLower(runes[index+1]) {
					flush()
				}
			}
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()
	return words
}

func snakeCase(name string) string {
	words := splitWords(name)
	for index, word := range words {
		words[index] = strings.ToLower(word)
	}
	return strings.Join(words, "_")
}

func pascalCase(name string) string {
	words := splitWords(name)
	for index, word := range words {
		lower := strings.ToLower(word)
		words[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, "")
}

func camelCase(name string) string {
	pascal := pascalCase(name)
	if pascal == "" {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// pluralize applies naive English pluralization, which covers typical
// resource names; pass an explicit table name when it guesses wrong.
func pluralize(word string) string {
	lower := strings.ToLower(word)
	switch {
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"), strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return word + "es"
	case len(word) > 1 && strings.HasSuffix(lower, "y") && !isVowel(rune(lower[len(lower)-2])):
		return word[:len(word)-1] + "ies"
	default:
		return word + "s"
	}
}

func isVowel(r rune) bool { return strings.ContainsRune("aeiou", r) }

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var pascalIdentifier = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// validateGeneratorName normalizes a user-supplied name and rejects anything
// that cannot become a valid exported Go identifier.
func validateGeneratorName(name string) (pascal, camel, snake string, err error) {
	pascal = pascalCase(name)
	if pascal == "" || !pascalIdentifier.MatchString(pascal) {
		return "", "", "", fmt.Errorf("name %q must normalize to a Go identifier such as Ticket or ApiKey", name)
	}
	camel = camelCase(name)
	if goKeywords[camel] {
		return "", "", "", fmt.Errorf("name %q collides with a Go keyword", name)
	}
	return pascal, camel, snakeCase(name), nil
}
