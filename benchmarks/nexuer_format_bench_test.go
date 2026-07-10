package benchmarks

import (
	"testing"

	"github.com/nexuer/log"
)

func BenchmarkNexuerHandlerFormats(b *testing.B) {
	b.Logf("Compare NexuerLog JSON and text handlers with matching scenarios.")
	for _, tt := range []struct {
		name     string
		logger   func() *log.Logger
		disabled func() *log.Logger
	}{
		{
			name:     "JSON",
			logger:   newNexuerLogger,
			disabled: newDisabledNexuerLogger,
		},
		{
			name:     "Text",
			logger:   newNexuerTextLogger,
			disabled: newDisabledNexuerTextLogger,
		},
	} {
		b.Run(tt.name+"/DisabledWithoutFields", func(b *testing.B) {
			logger := tt.disabled()
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0))
				}
			})
		})
		b.Run(tt.name+"/WithoutFields", func(b *testing.B) {
			logger := tt.logger()
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0))
				}
			})
		})
		b.Run(tt.name+"/AccumulatedContext", func(b *testing.B) {
			logger := tt.logger().With(fakeNexuerLogKvs()...)
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0))
				}
			})
		})
		b.Run(tt.name+"/AccumulatedContextWithValuer", func(b *testing.B) {
			logger := tt.logger().With(fakeNexuerLogKvs(true)...)
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0))
				}
			})
		})
		b.Run(tt.name+"/AddingFields", func(b *testing.B) {
			logger := tt.logger()
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0), fakeNexuerLogKvs()...)
				}
			})
		})
		b.Run(tt.name+"/FlatFields", func(b *testing.B) {
			logger := tt.logger().With("request_id", "req-1")
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0), "method", "GET", "status", 200)
				}
			})
		})
		b.Run(tt.name+"/WithGroup", func(b *testing.B) {
			logger := tt.logger().WithGroup("request").With("id", "req-1")
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					logger.InfoS(getMessage(0), "method", "GET", "status", 200)
				}
			})
		})
	}
}
