package nexssflow

import (
	"fmt"
	"sync/atomic"
	"testing"

	core "github.com/nexssp/lang/core"
)

var testLanguageIDSequence atomic.Uint64

func uniqueTestLanguageID(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("nexssflow-test-language-%d", testLanguageIDSequence.Add(1))
}

func registerCoreTestLanguage(t *testing.T, language core.Language) {
	t.Helper()
	if err := core.Register(language); err != nil {
		t.Fatalf("core.Register(%q): %v", language.ID, err)
	}
}
