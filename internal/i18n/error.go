package i18n

import "fmt"

// Localized is implemented by errors that carry an i18n key + args. Callers
// at the CLI boundary call Localize(loc) to render a human-readable message
// in the active locale. I18nKey / I18nArgs remain on the interface so
// callers that want to inspect or re-key the payload can still do so.
//
// All in-tree implementations embed [Payload], which provides Localize for
// free, so adding a new domain error type only requires composing Payload
// without re-implementing Localize.
type Localized interface {
	error
	I18nKey() string
	I18nArgs() map[string]any
	Localize(Locale) string
}

// Payload carries an i18n key and its arguments. Domain error types embed it
// (or compose it as a value field) so they all satisfy [Localized] without
// duplicating boilerplate.
type Payload struct {
	Key  string
	Args map[string]any
}

// NewPayload builds a Payload from alternating key/value pairs.
//
// args follows the same flat (string-key, value) varargs shape as T: pairs
// with a non-string key are silently dropped at the ToArgsMap boundary, and
// an odd-length args silently drops the trailing unpaired key. Callers must
// always pass complete (string-key, value) pairs.
func NewPayload(key string, args ...any) Payload {
	return Payload{Key: key, Args: ToArgsMap(args)}
}

// I18nKey reports the message key.
func (p Payload) I18nKey() string { return p.Key }

// I18nArgs returns the placeholder map.
func (p Payload) I18nArgs() map[string]any { return p.Args }

// Localize renders the payload into the given locale.
func (p Payload) Localize(loc Locale) string {
	return T(loc, p.Key, Flat(p.Args)...)
}

// String renders the payload for debug/log output.
func (p Payload) String() string { return fmt.Sprintf("%s: %v", p.Key, p.Args) }

// Flat converts an args map back into the alternating-key/value flattened
// slice that [T] consumes.
func Flat(m map[string]any) []any {
	out := make([]any, 0, len(m)*2)
	for k, v := range m {
		out = append(out, k, v)
	}
	return out
}
