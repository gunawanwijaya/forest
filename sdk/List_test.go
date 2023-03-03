package sdk_test

import (
	"fmt"
	"testing"

	. "github.com/gunawanwijaya/forest/sdk"
	. "github.com/onsi/gomega"
)

func test_List(t *testing.T) {
	t.Parallel()

	Expect := NewWithT(t).Expect
	l := List{}
	val := struct{}{}

	Expect(len(l)).To(BeZero())

	l.Add(val)
	Expect(len(l)).NotTo(BeZero())
	Expect(l[0]).To(Equal(val))

	l.Map(func(k int, v interface{}) { l[k] = fmt.Sprintf("%v", v) })
	Expect(l[0]).To(Equal("{}"))

	l.Delete(0)
	Expect(len(l)).To(BeZero())
}
