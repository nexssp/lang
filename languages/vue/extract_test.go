package vue

import (
	"strings"
	"testing"
)

func TestParseVueSnippets(t *testing.T) {
	t.Parallel()

	vueCode := []byte(`<script lang="ts">
export interface UserProps {
    name: string;
    role: string;
}

export function submitOrder(id: string): boolean {
    return true;
}

function internalHelper() {}
</script>

<template>
    <UserHeader title="Dashboard" />
    <OrderList :items="orders" />
    <div>
        <p>Text</p>
    </div>
</template>
`)

	snippets, funcCount := parseVue(vueCode, true)

	joined := strings.Join(snippets, "\n")
	if !strings.Contains(joined, "interface UserProps") {
		t.Errorf("expected UserProps interface from <script>")
	}
	if !strings.Contains(joined, "submitOrder") {
		t.Errorf("expected submitOrder function signature from <script>")
	}
	if strings.Contains(joined, "internalHelper") {
		t.Errorf("did not expect unexported internalHelper when exportedOnly=true")
	}
	if !strings.Contains(joined, "<UserHeader") || !strings.Contains(joined, "<OrderList") {
		t.Errorf("expected custom component tags in UI COMPONENTS, got:\n%s", joined)
	}
	if funcCount == 0 {
		t.Errorf("expected function count > 0, got %d", funcCount)
	}
}
