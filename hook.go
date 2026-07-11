package log

import "context"

// Replacer transforms user fields and the built-in level, msg, and logger
// fields. It can change a field's key or value; returning an empty Field removes
// the field. Groups is nil for built-in fields.
type Replacer func(ctx context.Context, groups []string, field Field) Field
