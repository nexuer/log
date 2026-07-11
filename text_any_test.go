package log

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"
)

func TestTextAnyMatchesFmtOutput(t *testing.T) {
	type namedStrings []string
	times := []time.Time{
		time.Date(2026, time.July, 11, 14, 30, 0, 123456789, time.FixedZone("CST", 8*60*60)),
		time.Now(),
	}
	tests := []struct {
		name  string
		value any
	}{
		{"nil ints", []int(nil)},
		{"empty ints", []int{}},
		{"ints", []int{0, -1, 42}},
		{"int64s", []int64{math.MinInt64, 0, math.MaxInt64}},
		{"uint64s", []uint64{0, math.MaxUint64}},
		{"float64s", []float64{0, -1.5, math.Inf(1), math.NaN()}},
		{"bools", []bool{true, false}},
		{"nil strings", []string(nil)},
		{"strings", []string{"", "plain", "with space", "quote\"", "line\nfeed", `back\\slash`}},
		{"times", times},
		{"named slice fallback", namedStrings{"a", "b"}},
		{"struct fallback", struct {
			ID   int
			Name string
		}{ID: 1, Name: "alice"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			New(&buf, Text()).InfoS("done", "value", tt.value)

			formatted := fmt.Sprintf("%+v", tt.value)
			if needsQuoting(formatted) {
				formatted = strconv.Quote(formatted)
			}
			want := "INFO msg=done value=" + formatted + "\n"
			if got := buf.String(); got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}
