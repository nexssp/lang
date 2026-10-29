package rust

import (
	"strings"
	"testing"
)

func TestParseRustSnippets(t *testing.T) {
	rustCode := []byte(`
pub struct ServiceConfig {
    pub host: String,
    port: u16,
}

pub trait Runner {
    fn execute(&self);
}

impl Runner for ServiceConfig {
    pub fn start(&self) {
        println!("Starting...");
    }

    fn internal_step(&self) {}
}

pub fn public_boot() -> bool {
    true
}

fn internal_boot() {}
`)

	t.Run("Rust Public AST Signatures", func(t *testing.T) {
		snippets, funcCount, err := parseRust(rustCode, true)
		if err != nil {
			t.Fatalf("parseRust failed: %v", err)
		}

		joined := strings.Join(snippets, "\n")

		if !strings.Contains(joined, "pub struct ServiceConfig") {
			t.Errorf("expected pub struct ServiceConfig")
		}
		if !strings.Contains(joined, "pub trait Runner") {
			t.Errorf("expected pub trait Runner")
		}
		if !strings.Contains(joined, "pub fn public_boot() -> bool {}") {
			t.Errorf("expected public_boot signature, got:\n%s", joined)
		}
		if strings.Contains(joined, "fn internal_boot()") {
			t.Errorf("did not expect private function internal_boot")
		}
		if funcCount < 2 {
			t.Errorf("expected at least 2 public functions, got %d", funcCount)
		}
	})
}

func TestParseRustExtendedFeatures(t *testing.T) {
	rustCode := []byte(`
pub mod network {
    pub fn connect() -> bool {
        true
    }
}

pub use network::connect;

macro_rules! define_handler {
    ($name:ident) => {
        pub fn $name() {}
    };
}

pub trait Service {
    type Context;
    const TIMEOUT: u32 = 5000;

    fn ping(&self);

    fn default_health_check(&self) -> bool {
        true
    }
}

impl Service for network::connect {
    type Context = String;
    const TIMEOUT: u32 = 1000;

    fn ping(&self) {
        println!("pong");
    }
}
`)

	snippets, funcCount, err := parseRust(rustCode, true)
	if err != nil {
		t.Fatalf("parseRust failed: %v", err)
	}

	joined := strings.Join(snippets, "\n")

	if !strings.Contains(joined, "macro_rules! define_handler { /* ... */ }") {
		t.Errorf("expected stripped macro signature, got:\n%s", joined)
	}
	if !strings.Contains(joined, "fn default_health_check(&self) -> bool {}") {
		t.Errorf("expected stripped default trait method, got:\n%s", joined)
	}
	if !strings.Contains(joined, "type Context = String;") || !strings.Contains(joined, "const TIMEOUT: u32 = 1000;") {
		t.Errorf("expected associated items in impl block, got:\n%s", joined)
	}
	if !strings.Contains(joined, "pub mod network") || !strings.Contains(joined, "pub use network::connect;") {
		t.Errorf("expected module and re-export declarations, got:\n%s", joined)
	}
	if funcCount < 3 {
		t.Errorf("expected at least 3 functions counted, got %d", funcCount)
	}
}
