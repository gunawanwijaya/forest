package sdk_test

import (
	"fmt"
	"testing"

	. "github.com/gunawanwijaya/forest/sdk"
	. "github.com/onsi/gomega"
)

func test_ListError(t *testing.T) {
	t.Parallel()

	Expect := NewWithT(t).Expect
	err := fmt.Errorf("error")
	errs := new(ListError)

	errs = errs.Add(err)
	Expect(errs.Err()).NotTo(BeNil())
	Expect(errs.Err().Error()).To(Equal(err.Error()))

	errs = errs.Add(err)
	Expect(errs.Err()).NotTo(BeNil())
	Expect(errs.Err().Error()).To(Equal(err.Error() + "\n" + err.Error()))

	errs = errs.Add(nil, nil, err, fmt.Errorf("error"))
	Expect(errs.Err()).NotTo(BeNil())
	Expect(errs.Err().Error()).To(Equal(err.Error() + "\n" + err.Error() + "\n" + err.Error() + "\n" + err.Error()))

	Expect(errs.Unwrap()).NotTo(BeNil())
	Expect(errs.Unwrap()).NotTo(BeNil())
	Expect(errs.Unwrap()).NotTo(BeNil())
	Expect(errs.Unwrap()).NotTo(BeNil())
	Expect(errs.Unwrap()).To(BeNil())

	errs = errs.Add(nil)
	Expect(errs.Err()).To(BeNil())
}
