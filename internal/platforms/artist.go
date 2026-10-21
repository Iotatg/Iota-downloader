package platforms

import (
	"regexp"
	"strings"
	"unicode"
)

var artistSeparator = regexp.MustCompile(`[,;|·]`)

// NormalizeArtistNames turns the many shapes an artist field can have into a
// single, comma separated artist string.
//
// It accepts a plain string, a list of names or a list of objects carrying a
// name, removes duplicates and keeps names such as "Tyler, The Creator" intact
// as long as they are not repeated.
func NormalizeArtistNames(value any) string {
	var raw []string

	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		raw = []string{typed}
	case []any:
		for _, item := range typed {
			switch entry := item.(type) {
			case string:
				raw = append(raw, entry)
			case map[string]any:
				name := stringValue(entry["name"])
				if name == "" {
					name = stringValue(entry["title"])
				}
				if name != "" {
					raw = append(raw, name)
				}
			}
		}
	case map[string]any:
		name := stringValue(typed["name"])
		if name != "" {
			raw = append(raw, name)
		}
	}

	if len(raw) == 0 {
		return ""
	}

	var cleaned []string
	for _, item := range raw {
		normalized := collapseWhitespace(item)
		if normalized != "" {
			cleaned = append(cleaned, normalized)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}

	seen := map[string]bool{}
	var unique []string
	for _, name := range cleaned {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, name)
	}

	if len(unique) == 1 {
		return unique[0]
	}

	// A single artist name may itself contain a comma, such as
	// "Tyler, The Creator". Keep it as it is when no repetition exists.
	if len(unique) == 2 && strings.Contains(unique[0], ",") && !strings.Contains(unique[1], ",") {
		if strings.EqualFold(strings.TrimSpace(unique[0]), strings.TrimSpace(unique[1])) {
			return unique[0]
		}
	}

	parts := artistSeparator.Split(unique[0], -1)
	if len(parts) == 2 {
		joined := strings.TrimSpace(parts[0]) + ", " + strings.TrimSpace(parts[1])
		for _, other := range unique[1:] {
			if strings.EqualFold(joined, other) {
				return joined
			}
		}
	}

	return strings.Join(unique, ", ")
}

func collapseWhitespace(value string) string {
	var builder strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && builder.Len() > 0 {
			builder.WriteRune(' ')
		}
		space = false
		builder.WriteRune(r)
	}
	return strings.TrimSpace(builder.String())
}

// SanitizeFilename turns an arbitrary title into a safe file name.
func SanitizeFilename(value string, maxBytes int) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r < 32:
			continue
		case strings.ContainsRune(`/\:*?"<>|`, r):
			builder.WriteRune('_')
		default:
			builder.WriteRune(r)
		}
	}
	cleaned := strings.TrimSpace(builder.String())
	cleaned = strings.Trim(cleaned, ".")
	if cleaned == "" {
		cleaned = "media"
	}
	if maxBytes > 0 && len(cleaned) > maxBytes {
		cleaned = strings.TrimSpace(cleaned[:maxBytes])
	}
	return cleaned
}
