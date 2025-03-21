package log

// Replacer transforms log fields to hide sensitive data or desensitize information.
type Replacer func(groups []string, field Field) Field
