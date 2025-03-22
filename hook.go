package log

import "context"

// Replacer transforms log fields to hide sensitive data or desensitize information.
type Replacer func(ctx context.Context, groups []string, field Field) Field
