package d

import "bytes"

type batchResponseRecorder struct {
	body bytes.Buffer
}

// Case H: One-liner method with unnamed multiple returns
func (r *batchResponseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) } // want "function Write returns error but missing defer errstk.Wrap\\(&err\\)"
