package llm

import (
	"log"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxResumeBytes         = 50 * 1024
	MaxJobDescriptionBytes = 15 * 1024
)

var injectionPatterns = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"ignore all instructions",
	"ignore the above",
	"disregard previous instructions",
	"disregard all instructions",
	"disregard the above",
	"forget your instructions",
	"forget all instructions",
	"forget the above",
	"override your instructions",
	"new instructions:",
	"system prompt:",
	"you are now",
	"pretend you are",
	"act as if",
	"do not follow",
	"do not obey",
	"reveal your system prompt",
	"reveal your instructions",
	"output your system prompt",
	"print your system prompt",
	"show your system prompt",
	"repeat your system prompt",
	"what is your system prompt",
	"what are your instructions",
}

var chatMLTokens = []string{
	"<|im_start|>",
	"<|im_end|>",
	"<|im_sep|>",
	"<|endoftext|>",
	"<|system|>",
	"<|user|>",
	"<|assistant|>",
	"</s>",
	"[INST]",
	"[/INST]",
	"<<SYS>>",
	"<</SYS>>",
}

var roleImpersonationRegex = regexp.MustCompile(`(?im)^\s*(System|Human|Assistant|User)\s*:\s*`)
var excessiveNewlineRegex = regexp.MustCompile(`\n{4,}`)
var excessiveSpaceRegex = regexp.MustCompile(`[ \t]{10,}`)

func SanitizeUserInput(text string) string {
	sanitized := text

	for _, token := range chatMLTokens {
		sanitized = strings.ReplaceAll(sanitized, token, "")
	}

	lowered := strings.ToLower(sanitized)
	for _, pattern := range injectionPatterns {
		idx := strings.Index(lowered, pattern)
		if idx >= 0 {
			log.Printf("[prompt-guard] stripped injection pattern: %q", pattern)
			sanitized = sanitized[:idx] + sanitized[idx+len(pattern):]
			lowered = strings.ToLower(sanitized)
		}
	}

	sanitized = roleImpersonationRegex.ReplaceAllStringFunc(sanitized, func(match string) string {
		log.Printf("[prompt-guard] stripped role impersonation: %q", strings.TrimSpace(match))
		return ""
	})

	sanitized = excessiveNewlineRegex.ReplaceAllString(sanitized, "\n\n\n")
	sanitized = excessiveSpaceRegex.ReplaceAllString(sanitized, " ")

	return strings.TrimSpace(sanitized)
}

func ValidateInputLength(text string, maxBytes int, label string) error {
	if len(text) > maxBytes {
		return &InputTooLargeError{
			Label:    label,
			MaxBytes: maxBytes,
			Got:      len(text),
		}
	}
	return nil
}

type InputTooLargeError struct {
	Label    string
	MaxBytes int
	Got      int
}

func (e *InputTooLargeError) Error() string {
	maxKB := e.MaxBytes / 1024
	gotKB := e.Got / 1024
	return e.Label + " is too large (" + itoa(gotKB) + "KB); maximum allowed is " + itoa(maxKB) + "KB"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func WrapUserData(tag, content string) string {
	_ = utf8.ValidString(content)
	return "<" + tag + ">\n" + content + "\n</" + tag + ">"
}
