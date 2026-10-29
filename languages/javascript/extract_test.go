package javascript

import (
	"strings"
	"testing"
)

func TestParseTypeScriptSnippets(t *testing.T) {
	tsCode := []byte(`
export interface UserConfig {
	id: string;
	apiKey?: string;
}

interface InternalConfig {
	hidden: boolean;
}

export function AuthenticateUser(token: string): boolean {
	return token.length > 0;
}

function internalHelper() {
	return 42;
}

export const ProcessPayment = async (amount: number) => {
	await doPayment(amount);
};
`)

	snippets, funcCount, err := parseTSJS(tsCode, "test.ts", true)
	if err != nil {
		t.Fatalf("parseTSJS failed: %v", err)
	}

	joined := strings.Join(snippets, "\n")
	if !strings.Contains(joined, "export interface UserConfig") {
		t.Errorf("expected UserConfig export")
	}
	if strings.Contains(joined, "InternalConfig") {
		t.Errorf("expected InternalConfig to be skipped")
	}
	if !strings.Contains(joined, "export function AuthenticateUser") {
		t.Errorf("expected AuthenticateUser signature")
	}
	if strings.Contains(joined, "internalHelper") {
		t.Errorf("expected internalHelper to be skipped")
	}
	if funcCount < 2 {
		t.Errorf("expected at least 2 functions, got %d", funcCount)
	}
}

func TestParseTSXComponentSnippets(t *testing.T) {
	tsxCode := []byte(`
import React from 'react';

export interface ButtonProps {
    label: string;
    onClick: () => void;
}

export const PrimaryButton: React.FC<ButtonProps> = (props) => {
    return <button onClick={props.onClick}>{props.label}</button>;
};

export class UIController {
    public mount() {
        console.log("mounted");
    }
}
`)

	snippets, funcCount, err := parseTSJS(tsxCode, "Component.tsx", true)
	if err != nil {
		t.Fatalf("parseTSJS for TSX failed: %v", err)
	}

	joined := strings.Join(snippets, "\n")
	if !strings.Contains(joined, "export interface ButtonProps") {
		t.Errorf("expected interface ButtonProps in output:\n%s", joined)
	}
	if !strings.Contains(joined, "export const PrimaryButton") {
		t.Errorf("expected PrimaryButton in output:\n%s", joined)
	}
	if !strings.Contains(joined, "class UIController") {
		t.Errorf("expected class UIController in output:\n%s", joined)
	}
	if funcCount == 0 {
		t.Errorf("expected at least 1 function found")
	}
}

func TestParseTypeScriptExportsAndClasses(t *testing.T) {
	t.Parallel()

	tsCode := []byte(`
export interface Repository<T> {
	findById(id: string): Promise<T | null>;
	save(entity: T): Promise<void>;
}

export type ID = string | number;

export enum UserRole {
	Admin = "ADMIN",
	User = "USER"
}

export class UserService {
	constructor(private repo: Repository<User>) {}

	public async getUser(id: ID): Promise<User | null> {
		return this.repo.findById(String(id));
	}
}

export const CalculateTax = async (amount: number): Promise<number> => {
	return amount * 0.2;
};

const internalLocal = 123;
`)

	snippets, funcCount, err := parseTSJS(tsCode, "service.ts", true)
	if err != nil {
		t.Fatalf("parseTSJS failed: %v", err)
	}

	joined := strings.Join(snippets, "\n\n")

	if !strings.Contains(joined, "export interface Repository<T>") {
		t.Errorf("expected generic Repository interface with export")
	}
	if !strings.Contains(joined, "export type ID = string | number;") {
		t.Errorf("expected exported type alias ID")
	}
	if !strings.Contains(joined, "export enum UserRole") {
		t.Errorf("expected exported enum UserRole")
	}
	if !strings.Contains(joined, "export class UserService {") {
		t.Errorf("expected export class UserService")
	}
	if !strings.Contains(joined, "constructor(private repo: Repository<User>) {}") {
		t.Errorf("expected constructor parameter properties preserved")
	}
	if !strings.Contains(joined, "public async getUser(id: ID): Promise<User | null> {}") {
		t.Errorf("expected public async method signature")
	}
	if !strings.Contains(joined, "export const CalculateTax = async (amount: number): Promise<number> => {}") {
		t.Errorf("expected exported arrow function signature, got:\n%s", joined)
	}
	if strings.Contains(joined, "internalLocal") {
		t.Errorf("did not expect unexported constant when exportedOnly=true")
	}
	if funcCount != 3 { // constructor, getUser, CalculateTax
		t.Errorf("expected 3 functions, got %d", funcCount)
	}
}
