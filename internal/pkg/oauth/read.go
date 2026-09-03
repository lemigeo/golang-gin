package oauth

import "io"

// readAtMost reads up to limit bytes and fails if the body is longer, so an
// unbounded provider response cannot exhaust memory.
func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, io.ErrShortBuffer
	}
	return b, nil
}
