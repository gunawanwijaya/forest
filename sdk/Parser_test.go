package sdk_test

import (
	"encoding/xml"
	"testing"

	. "github.com/gunawanwijaya/forest/sdk"
	. "github.com/onsi/gomega"
)

func test_Parser(t *testing.T) {
	t.Parallel()

	Expect := NewWithT(t).Expect

	raw := []byte(`
	<a>
		<b>red</b>
		<c>blue</c>
	</a>
	`)
	model := struct {
		XMLName xml.Name `xml:"a"`
		B       string   `xml:"b"`
		C       string   `xml:"c"`
	}{}
	Expect(XML.Unmarshal(raw, &model)).To(Succeed())
	Expect(model.B).To(Equal("red"))
	Expect(model.C).To(Equal("blue"))
}
