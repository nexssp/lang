package php

import (
	"strings"
	"testing"
)

func TestParsePHPSnippets(t *testing.T) {
	phpCode := []byte(`<?php
namespace App\Services;

function standaloneHelper($param) {
    return $param * 2;
}

class PaymentGateway {
    public string $apiUrl;
    private string $apiKey;

    public function process(float $amount): bool {
        return true;
    }

    protected function internalVerify(): void {}
    private function logSecret(): void {}
}
`)

	t.Run("PHP Exported (Public) Only", func(t *testing.T) {
		snippets, funcCount, err := parsePHP(phpCode, true)
		if err != nil {
			t.Fatalf("parsePHP failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")
		if !strings.Contains(joined, "public string $apiUrl;") {
			t.Errorf("expected public property $apiUrl, got:\n%s", joined)
		}
		if strings.Contains(joined, "$apiKey") {
			t.Errorf("did not expect private $apiKey in exported_only output")
		}
		if !strings.Contains(joined, "public function process(float $amount): bool {}") {
			t.Errorf("expected public method process signature stripped of body, got:\n%s", joined)
		}
		if strings.Contains(joined, "internalVerify") || strings.Contains(joined, "logSecret") {
			t.Errorf("did not expect non-public methods in exported_only output")
		}
		if funcCount == 0 {
			t.Errorf("expected at least 1 function counted, got %d", funcCount)
		}
	})
}
