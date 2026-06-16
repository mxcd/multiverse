package brain

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// kebabRe matches a single, already-valid kebab-case path segment: lowercase
// ASCII letters/digits in groups separated by single hyphens.
var kebabRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// slugExpand handles the few letters Unicode decomposition won't fold to ASCII
// on its own. Applied before diacritic stripping so German notes read sensibly.
var slugExpand = strings.NewReplacer(
	"ß", "ss", "ẞ", "ss",
	"æ", "ae", "Æ", "ae",
	"œ", "oe", "Œ", "oe",
	"ø", "o", "Ø", "o",
	"đ", "d", "Đ", "d",
	"ð", "d", "Ð", "d",
	"þ", "th", "Þ", "th",
	"ł", "l", "Ł", "l",
	"&", " and ",
)

// diacriticFold decomposes accented characters (NFKD) and drops the combining
// marks, turning "é" into "e", "ü" into "u", etc.
var diacriticFold = transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Slugify converts a single name segment to kebab-case: lowercase ASCII letters
// and digits, words joined by single hyphens. Accents are folded to ASCII and
// every other character collapses to a separator. It returns "" only for input
// that contains no slug-able characters (callers substitute a fallback).
//
// This is the canonical filename form the brain enforces — chosen so names are
// portable across case-insensitive filesystems (Windows, macOS) and safe to type
// unquoted in a shell.
func Slugify(s string) string {
	s = slugExpand.Replace(s)
	if folded, _, err := transform.String(diacriticFold, s); err == nil {
		s = folded
	}
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevHyphen = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// slugOrUntitled is Slugify with a fallback for segments that slug to nothing.
func slugOrUntitled(s string) string {
	if out := Slugify(s); out != "" {
		return out
	}
	return "untitled"
}

// SlugifyPath kebab-cases every segment of a vault-relative path, preserving the
// "/" separators and a trailing ".md" extension. Empty and "." segments drop.
func SlugifyPath(rel string) string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	ext := ""
	if len(rel) >= 3 && strings.EqualFold(rel[len(rel)-3:], ".md") {
		ext = ".md"
		rel = rel[:len(rel)-3]
	}
	var out []string
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." {
			continue
		}
		out = append(out, slugOrUntitled(seg))
	}
	return strings.Join(out, "/") + ext
}

// IsKebab reports whether a single path segment is already valid kebab-case.
func IsKebab(seg string) bool { return kebabRe.MatchString(seg) }

// pathSegments returns the directory and filename-stem segments of a
// vault-relative note path (the ".md" extension excluded).
func pathSegments(rel string) []string {
	rel = strings.TrimSuffix(filepath.ToSlash(rel), ".md")
	return strings.Split(rel, "/")
}

// IsKebabPath reports whether every segment of a vault-relative note path is
// already kebab-case (the ".md" extension excluded), and returns the first
// offending segment when not.
func IsKebabPath(rel string) (string, bool) {
	for _, seg := range pathSegments(rel) {
		if !IsKebab(seg) {
			return seg, false
		}
	}
	return "", true
}

// slugKey is the canonical match key for a note name or wikilink target: the
// kebab slug of its base name, with any ".md" extension removed. Two references
// that name the same note share a slug key regardless of case or spacing, so it
// is what link resolution and the backlink graph compare on.
func slugKey(name string) string {
	base := filepath.Base(filepath.ToSlash(strings.TrimSpace(name)))
	base = strings.TrimSuffix(base, ".md")
	return Slugify(base)
}
