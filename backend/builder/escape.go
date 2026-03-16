package builder

import "strings"

// EscapeLatex escapes special LaTeX characters so user-provided text can be
// safely interpolated into a LaTeX document without breaking compilation.
func EscapeLatex(s string) string {
	r := strings.NewReplacer(
		`\`, `\textbackslash{}`,
		`&`, `\&`,
		`%`, `\%`,
		`$`, `\$`,
		`#`, `\#`,
		`_`, `\_`,
		`{`, `\{`,
		`}`, `\}`,
		`~`, `\textasciitilde{}`,
		`^`, `\textasciicircum{}`,
	)
	return r.Replace(s)
}
