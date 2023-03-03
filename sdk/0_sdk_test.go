package sdk_test

import (
	"testing"
)

func Test_Suite_SDK(t *testing.T) {
	// t.Run("Cipher", test_Cipher)
	// t.Run("Currency", test_Currency)
	// t.Run("Dict", test_Dict)
	// t.Run("Flags", test_Flags)
	// t.Run("Generator", test_Generator)
	t.Run("HTTPMux", test_HTTPMux)
	t.Run("List", test_List)
	t.Run("ListError", test_ListError)
	t.Run("OpenTelemetry", test_OpenTelemetry)
	t.Run("Parser", test_Parser)
	// t.Run("PhoneNumber", test_PhoneNumber)
	// t.Run("SourceError", test_SourceError)
}
