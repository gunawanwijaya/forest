package sdk_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	. "github.com/gunawanwijaya/forest/sdk"
	. "github.com/onsi/gomega"
)

func test_OpenTelemetry(t *testing.T) {
	otel := OTel
	ctx := context.Background()

	t.Parallel()

	Expect := NewWithT(t).Expect
	buf := new(bytes.Buffer)
	length := 64
	b64 := base64.RawStdEncoding
	btoa := func(b []byte) (enc []byte) {
		enc = make([]byte, b64.EncodedLen(len(b)))
		b64.Encode(enc, b)

		return
	}
	nonce := func(length int) []byte {
		nonce := make([]byte, length)
		_, _ = io.ReadFull(rand.Reader, nonce)

		return nonce
	}
	expected := func(msg []byte) []byte {
		return []byte(`{"message":"` + string(msg) + `"}` + "\n")
	}

	func(t *testing.T) {
		x := otel.NewLogger(ctx).Level("debug")

		msg := btoa(nonce(length))
		n, err := x.Z().Write(msg)

		Expect(err).Should(Succeed())
		Expect(n).Should(Equal(len(msg)))
	}(t)

	func(t *testing.T) {
		x := otel.NewLogger(ctx, buf)

		defer func() { buf.Reset() }()

		msg := btoa(nonce(length))
		exp := expected(msg)
		n, err := x.Z().Write(msg)

		Expect(err).Should(Succeed())
		Expect(n).Should(Equal(len(msg)))
		Expect(buf.String()).Should(Equal(string(exp)))
		Expect(buf.Bytes()).Should(Equal(exp))
	}(t)

	func(t *testing.T) {
		buf2 := new(bytes.Buffer)
		msg := btoa(nonce(length))
		exp := expected(msg)

		file, err := os.CreateTemp(t.TempDir(), "log-*.log")
		Expect(err).Should(Succeed())
		Expect(file.Name()).ShouldNot(BeEmpty())

		info, err := file.Stat()
		Expect(err).Should(Succeed())
		Expect(info).ShouldNot(BeNil())
		Expect(file.Name()).Should(ContainSubstring(info.Name()))
		Expect(strings.HasSuffix(file.Name(), info.Name())).Should(BeTrue())
		Expect(info.Size()).Should(Equal(int64(0)))
		Expect(info.Mode()).Should(Equal(fs.FileMode(0o600)))
		Expect(info.IsDir()).ShouldNot(BeTrue())
		Expect(info.Sys()).ShouldNot(BeNil())

		x := otel.NewLogger(ctx,
			file,
			buf,
			buf2,
			OTel.NewConsoleWriter(io.Discard),
		)

		n, err := x.Z().Write(msg)
		Expect(err).Should(Succeed())
		Expect(n).Should(Equal(len(msg)))

		// *os.File eventhough implement io.Reader, are unable to read using
		// anything that use io.Reader; so this case using os.ReadFile.
		fileBytes, err := os.ReadFile(file.Name())
		Expect(err).Should(Succeed())
		Expect(len(fileBytes)).Should(Equal(len(exp)))
		Expect(fileBytes).Should(Equal(exp))

		Expect(buf.String()).Should(Equal(string(exp)))
		Expect(buf.Bytes()).Should(Equal(exp))

		Expect(buf2.String()).Should(Equal(string(exp)))
		Expect(buf2.Bytes()).Should(Equal(exp))
	}(t)
}
