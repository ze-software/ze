// Design: docs/architecture/container/image-store.md -- the registry requests: a blob checked against its digest and size as it arrives
// Related: client.go -- the requests a blob is fetched through
// Related: manifest.go -- the descriptors a blob is named by

package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// copyBufferSize is the chunk a blob is read in.
const copyBufferSize = 32 << 10

// Compression is how a layer blob is compressed.
type Compression uint8

const (
	// CompressionUnspecified is no layer type. No Layer carries it.
	CompressionUnspecified Compression = iota
	CompressionNone
	CompressionGzip
	CompressionZstd
)

// Blob names one object of a repository by its digest and its declared size.
// Only this package builds one, from a descriptor it checked, so the size is
// never negative and the digest is SHA-256.
type Blob struct {
	digest Digest
	size   int64
}

// Digest answers the digest the blob's bytes must hash to.
func (b Blob) Digest() Digest { return b.digest }

// Size answers the size the blob's bytes must have.
func (b Blob) Size() int64 { return b.size }

// Layer is one layer of an image: its blob and how the blob is compressed.
type Layer struct {
	blob        Blob
	compression Compression
}

// Blob answers the layer's blob.
func (l Layer) Blob() Blob { return l.blob }

// Compression answers how the layer's blob is compressed.
func (l Layer) Compression() Compression { return l.compression }

// BlobFile is where FetchBlob writes a blob: the store's staging file.
// *os.File is one. Each attempt rewinds and empties it first, so a retry, or
// the registry after a failed mirror, never appends to a partial copy.
type BlobFile interface {
	io.Writer
	io.Seeker
	Truncate(size int64) error
}

// FetchBlob writes the blob of ref's repository into dst, checking each byte
// against the blob's declared size as it arrives and the whole against its
// digest at the end. When it answers an error, dst is left empty: no byte the
// digest did not vouch for stays where the store would read it.
func (c *Client) FetchBlob(ctx context.Context, ref Reference, blob Blob, dst BlobFile) error {
	if err := c.owns(ref); err != nil {
		return err
	}
	restart := func() error {
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return err
		}
		return dst.Truncate(0)
	}
	err := c.fetchBlob(ctx, ref, blob, dst, restart)
	if err == nil {
		return nil
	}
	if emptied := restart(); emptied != nil {
		return errors.Join(err, fmt.Errorf("the partial copy of %s could not be emptied: %w", blob.digest, emptied))
	}
	return err
}

// fetchConfig reads the image configuration blob, which imageParts bounded
// to configSizeMax, into memory.
func (c *Client) fetchConfig(ctx context.Context, ref Reference, blob Blob) ([]byte, error) {
	var config bytes.Buffer
	restart := func() error {
		config.Reset()
		return nil
	}
	if err := c.fetchBlob(ctx, ref, blob, &config, restart); err != nil {
		return nil, err
	}
	return config.Bytes(), nil
}

// fetchBlob asks the mirror, then the registry, for blob, restarting dst
// before each attempt.
func (c *Client) fetchBlob(ctx context.Context, ref Reference, blob Blob, dst io.Writer, restart func() error) error {
	path := "/v2/" + ref.repository + "/blobs/" + blob.digest.String()
	return c.fromMirrorThenRegistry(ctx, func(o *origin) error {
		if err := restart(); err != nil {
			return fmt.Errorf("the copy of %s cannot be restarted: %w", blob.digest, err)
		}
		got, err := c.get(ctx, o, path, "", pullScope(ref))
		if err != nil {
			return err
		}
		defer got.close()
		return copyVerified(ctx, o.host, dst, got.body, blob)
	})
}

// copyVerified copies body into dst while it hashes it. It stops before the
// first byte past the declared size, refuses a body shorter than it, and
// refuses a body whose hash is not the digest. A mismatch is never retried:
// another attempt would fetch another file.
func copyVerified(ctx context.Context, host string, dst io.Writer, body io.Reader, blob Blob) error {
	hash := sha256.New()
	buffer := make([]byte, copyBufferSize)
	var received int64
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			if received+int64(n) > blob.size {
				return mismatch(host, blob, "it holds more bytes than its descriptor declares", fmt.Errorf("declared %d bytes", blob.size))
			}
			received += int64(n)
			hash.Write(buffer[:n]) //nolint:errcheck // a hash never fails a write
			if _, err := dst.Write(buffer[:n]); err != nil {
				return fmt.Errorf("the copy of %s cannot be written: %w", blob.digest, err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return stopped(ctx)
			}
			return &Error{failure: FailureUnreachable, problem: "the transfer of " + blob.digest.String() + " from " + host + " stopped", why: "the registry stopped sending before the end of the blob", cause: readErr, retry: true}
		}
	}
	if received < blob.size {
		return mismatch(host, blob, "it holds fewer bytes than its descriptor declares", fmt.Errorf("received %d bytes, declared %d", received, blob.size))
	}
	if got := digestFromHash(hash.Sum(nil)); got != blob.digest {
		return mismatch(host, blob, "its bytes hash to "+got.String()+", not to the digest asked for", nil)
	}
	return nil
}

// mismatch builds the refusal of an object that is not what its descriptor
// names.
func mismatch(host string, blob Blob, why string, cause error) *Error {
	return &Error{failure: FailureDigestMismatch, problem: blob.digest.String() + " from " + host + " does not match its descriptor", why: why, cause: cause}
}

// digestOf answers the digest of data.
func digestOf(data []byte) Digest {
	sum := sha256.Sum256(data)
	return digestFromHash(sum[:])
}

// digestFromHash answers the digest of a SHA-256 sum.
func digestFromHash(sum []byte) Digest {
	return Digest{hex: hex.EncodeToString(sum)}
}

// pullScope answers the token scope of a pull from ref's repository.
func pullScope(ref Reference) string {
	return "repository:" + ref.repository + ":pull"
}

// owns refuses a reference of another registry: a client fetches from the
// one registry its configuration names, so a mismatch is a Ze defect.
func (c *Client) owns(ref Reference) error {
	if ref.registry != c.registryName {
		return errors.New("registry client: the reference names the registry " + ref.registry + ", and this client is for " + c.registryName)
	}
	return nil
}
