package log

import (
	"errors"
	"io"
)

type writerWrapper struct {
	io.Writer
}

func (w writerWrapper) Close() error {
	return nil
}

var discard = writerWrapper{Writer: io.Discard}

// addWriteCloser converts an io.Writer to a WriteSyncer. It attempts to be
// intelligent: if the concrete type of the io.Writer implements WriteSyncer,
// we'll use the existing Sync method. If it doesn't, we'll add a no-op Sync.
func addWriteCloser(w io.Writer) io.WriteCloser {
	if w == nil {
		return nil
	}
	switch nw := w.(type) {
	case io.WriteCloser:
		return nw
	default:
		return writerWrapper{w}
	}
}

type multiWriteCloser struct {
	writers []io.Writer
}

func (t multiWriteCloser) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// Close on all the underlying writers that are io.Closers. If any of the
// Close methods return an error, the remainder of the closers are not closed
// and the error is returned.
func (t multiWriteCloser) Close() error {
	for _, w := range t.writers {
		if closer, ok := w.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// MultiWriteCloser creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
//
// Each write is written to each listed writer, one at a time.
// If a listed writer returns an error, that overall write operation
// stops and returns the error; it does not continue down the list.
func MultiWriteCloser(writers ...io.Writer) io.WriteCloser {
	allWriters := make([]io.Writer, 0, len(writers))
	for _, w := range writers {
		if mw, ok := w.(*multiWriteCloser); ok {
			allWriters = append(allWriters, mw.writers...)
		} else if mw, ok := w.(*tryMultiWriteCloser); ok {
			allWriters = append(allWriters, mw.writers...)
		} else {
			allWriters = append(allWriters, w)
		}
	}
	return &multiWriteCloser{allWriters}
}

// ByteCountStrategy defines the strategy for determining the number of bytes returned by Write.
type ByteCountStrategy int

const (
	// StrategyFirst returns the byte count from the first writer.
	StrategyFirst ByteCountStrategy = iota
	// StrategyMin returns the minimum byte count among writes.
	StrategyMin
	// StrategyMax returns the maximum byte count among writes.
	StrategyMax
)

type tryMultiWriteCloser struct {
	writers           []io.Writer
	byteCountStrategy ByteCountStrategy
}

// Write attempts to write p to all underlying writers, collecting any errors.
// The returned byte count is determined by the byteCountStrategy:
// - StrategyFirst: first writer's byte count.
// - StrategyMinSuccess: minimum byte count among successful writes.
// - StrategyMaxSuccess: maximum byte count among successful writes.
func (t *tryMultiWriteCloser) Write(p []byte) (n int, err error) {
	var errs []error
	firstN := 0
	minN := len(p)
	maxN := 0
	for i, w := range t.writers {
		n, err = w.Write(p)
		if i == 0 {
			firstN = n // Record the first writer's byte count
		}
		if n < minN {
			minN = n
		}
		if n > maxN {
			maxN = n
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if n != len(p) {
			errs = append(errs, io.ErrShortWrite)
			continue
		}
	}

	switch t.byteCountStrategy {
	case StrategyMin:
		return minN, errors.Join(errs...)
	case StrategyMax:
		return maxN, errors.Join(errs...)
	default:
		return firstN, errors.Join(errs...)
	}
}

func (t *tryMultiWriteCloser) Close() error {
	var errs []error
	for _, w := range t.writers {
		if wc, ok := w.(io.Closer); ok {
			if err := wc.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// TryMultiWriteCloser creates a writer that attempts to write to all provided io.Writers.
// The first argument specifies the byte count strategy, which determines how the returned
// byte count is calculated. It collects all errors and joins them with errors.Join.
func TryMultiWriteCloser(strategy ByteCountStrategy, writers ...io.Writer) io.Writer {
	allWriters := make([]io.Writer, 0, len(writers))
	for _, w := range writers {
		if mw, ok := w.(*multiWriteCloser); ok {
			allWriters = append(allWriters, mw.writers...)
		} else if mw, ok := w.(*tryMultiWriteCloser); ok {
			allWriters = append(allWriters, mw.writers...)
		} else {
			allWriters = append(allWriters, w)
		}
	}
	return &tryMultiWriteCloser{allWriters, strategy}
}
