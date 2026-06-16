package brain

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Formula Student":            "formula-student",
		"already-kebab":              "already-kebab",
		"  Trim  Me  ":               "trim-me",
		"Stalwart v0.16 Migration":   "stalwart-v0-16-migration",
		"Braavos / Primestone NAM":   "braavos-primestone-nam",
		"Working with MaPa":          "working-with-mapa",
		"Café Müller über Straße":    "cafe-muller-uber-strasse",
		"R&D notes":                  "r-and-d-notes",
		"---weird___punctuation!!!":  "weird-punctuation",
		"UPPER":                      "upper",
		"Mixed_CASE-with.dots":       "mixed-case-with-dots",
		"":                           "",
		"!!!":                        "",
		"DeepThought — Session Hook": "deepthought-session-hook",
		"CLI is `verse`":             "cli-is-verse",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugifyPath(t *testing.T) {
	cases := map[string]string{
		"Projects/Formula Student.md": "projects/formula-student.md",
		"Code of Conduct.md":          "code-of-conduct.md",
		"Templates/Reference Note.md": "templates/reference-note.md",
		"already/kebab-case.md":       "already/kebab-case.md",
		"README.MD":                   "readme.md",
		"A/B/C/deep note.md":          "a/b/c/deep-note.md",
		"no-extension":                "no-extension",
	}
	for in, want := range cases {
		if got := SlugifyPath(in); got != want {
			t.Errorf("SlugifyPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsKebabPath(t *testing.T) {
	kebab := []string{"a.md", "projects/formula-student.md", "a/b/c.md", "x1/y2-z3.md"}
	for _, p := range kebab {
		if seg, ok := IsKebabPath(p); !ok {
			t.Errorf("IsKebabPath(%q) = false (seg %q), want true", p, seg)
		}
	}
	notKebab := map[string]string{
		"Projects/foo.md":     "Projects",
		"projects/Foo Bar.md": "Foo Bar",
		"README.md":           "README",
		"a/b/C.md":            "C",
	}
	for p, wantSeg := range notKebab {
		if seg, ok := IsKebabPath(p); ok || seg != wantSeg {
			t.Errorf("IsKebabPath(%q) = (%q, %v), want (%q, false)", p, seg, ok, wantSeg)
		}
	}
}
