package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nexssp/kernel/xerr"
)

// RunnerSpec is the data model parsed from a @lang.runner block in a .nflow
// file. One RunnerSpec per (language, version). The Go side is generic —
// adding a language means shipping a runner profile and optional template
// asset, with no runtime-specific Go changes.
//
// Native profile example:
//
//	@description "Python runner with JSON streams"
//	@lang.runner "python" {
//	  ext: ".py"
//	  aliases: ["py", "python"]
//	  find: ["python3", "python", "py -3"]
//	  stdin: "json"
//	  stdout: "json"
//	  file_template_file: "templates/python.py"
//	}
type RunnerSpec struct {
	ID               string   `json:"id,omitempty"`
	Version          string   `json:"version,omitempty"`
	Ext              string   `json:"ext"`
	Aliases          []string `json:"aliases,omitempty"`
	Find             []string `json:"find"`
	Stdin            string   `json:"stdin,omitempty"`
	Stdout           string   `json:"stdout,omitempty"`
	FileTemplate     string   `json:"file_template,omitempty"`
	FileTemplateFile string   `json:"file_template_file,omitempty"`
	FilePathMode     string   `json:"file_path_mode,omitempty"`
	Install          OSRecipe `json:"install,omitzero"`
	Deps             string   `json:"deps,omitempty"`
	DepsMarker       string   `json:"deps_marker,omitempty"`
}

// OSRecipe holds per-OS install commands. Each entry is a full command
// line; the first element is the binary, the rest are args.
type OSRecipe struct {
	Windows []string `json:"windows,omitempty"`
	Darwin  []string `json:"darwin,omitempty"`
	Linux   []string `json:"linux,omitempty"`
}

const windowsGOOS = "windows"

// RecipeFor returns the install command for the current OS, or nil if no
// recipe is defined. Pure Go: callers run it via os/exec.
func (r OSRecipe) RecipeFor() []string {
	switch runtime.GOOS {
	case windowsGOOS:
		return r.Windows
	case "darwin":
		return r.Darwin
	case "linux":
		return r.Linux
	default:
		return nil
	}
}

// Encode resolves the stdin codec name to a function.
func (s RunnerSpec) Encode() (func(any) ([]byte, error), error) {
	switch s.Stdin {
	case "", "json":
		return encodeJSON, nil
	case "text", "raw":
		return encodeText, nil
	default:
		return nil, xerr.Validation(fmt.Sprintf("%s: unknown stdin codec %q (want json|text|raw)", s.ID, s.Stdin))
	}
}

// Decode resolves the stdout codec name to a function.
func (s RunnerSpec) Decode() (func(string) (any, error), error) {
	switch s.Stdout {
	case "", "json":
		return decodeJSON, nil
	case "text", "raw":
		return decodeText, nil
	default:
		return nil, xerr.Validation(fmt.Sprintf("%s: unknown stdout codec %q (want json|text|raw)", s.ID, s.Stdout))
	}
}

// FindCmd tries each entry in Find via lookPathCmd. The first candidate that
// resolves to a usable interpreter wins; on Windows, py.exe is probed with
// its selected version and WSL is probed for a registered Bash distribution.
//
// Each Find entry is split on whitespace; the first token is the binary, the
// rest are pre-args (for example, "py -3" becomes cmd="py", args=["-3"]).
// The materialized source path is appended later by the runtime.
func (s RunnerSpec) FindCmd() (cmd string, args []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.FindCmdContext(ctx)
}

// FindCmdContext probes interpreter candidates while honoring cancellation.
func (s RunnerSpec) FindCmdContext(ctx context.Context) (cmd string, args []string, err error) {
	if ctx == nil {
		return "", nil, xerr.Validation("runner selection context is required")
	}
	return s.findCmd(lookPathCmd, func(cmd string, args []string) bool {
		return s.commandUsable(cmd, args, func(command string, arguments ...string) error {
			return runProbe(ctx, command, arguments...)
		})
	})
}

func (s RunnerSpec) commandUsable(cmd string, args []string, probe func(string, ...string) error) bool {
	if runtime.GOOS == windowsGOOS && s.FilePathMode == "wsl" && isWindowsWSLLauncher(cmd) {
		return probe(cmd, "--exec", "bash", "-lc", "command -v wslpath >/dev/null 2>&1") == nil
	}
	if runtime.GOOS != windowsGOOS || !isWindowsPythonLauncher(cmd) {
		return true
	}
	probeArgs := append(append([]string(nil), args...), "--version")
	return probe(cmd, probeArgs...) == nil
}

func runProbe(ctx context.Context, cmd string, args ...string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return exec.CommandContext(probeCtx, cmd, args...).Run()
}

func (s RunnerSpec) findCmd(lookPath func(string) (string, error), usable func(string, []string) bool) (cmd string, args []string, err error) {
	for _, entry := range s.Find {
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}
		if foundCmd, lookupErr := lookPath(parts[0]); lookupErr == nil && usable(foundCmd, parts[1:]) {
			return foundCmd, parts[1:], nil
		}
	}
	return "", nil, ErrNotFound
}

func isWindowsPythonLauncher(cmd string) bool {
	cmd = strings.ToLower(strings.ReplaceAll(cmd, `\`, "/"))
	return path.Base(cmd) == "py.exe"
}

func isWindowsWSLLauncher(cmd string) bool {
	cmd = strings.ToLower(strings.ReplaceAll(cmd, `\`, "/"))
	base := path.Base(cmd)
	return base == "wsl.exe" || base == "wsl"
}

// --- embedded runners ----------------------------------------------------

func init() {
	if err := LoadRunners(); err != nil {
		panic("lang: " + err.Error())
	}
}

// RunnerFiles returns the embedded runner profile filesystem. Paths are rooted
// at "runners/" so adapters can expose the profiles as source modules.
func RunnerFiles() fs.FS { return runnersFS }

// LoadRunners parses every runners/*.nflow profile from the embed and registers
// each as a Language. The package calls this once at init; manual repeated
// calls return a duplicate-registration error.
func LoadRunners() error {
	entries, err := runnersFS.ReadDir("runners")
	if err != nil {
		return xerr.Internal("lang: read runners directory", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nflow") {
			continue
		}
		raw, err := runnersFS.ReadFile(path.Join("runners", e.Name()))
		if err != nil {
			return xerr.Internal("lang: read runner profile "+e.Name(), err)
		}
		spec, err := ParseRunner(string(raw))
		if err != nil {
			return xerr.BadRequest("lang: parse runner profile "+e.Name(), err)
		}
		if err := safeRegisterSpec(spec); err != nil {
			return xerr.Conflict("lang: register runner profile "+e.Name(), err)
		}
	}
	return nil
}

// safeRegisterSpec wraps registerSpec with a recover, converting unexpected
// invariant panics to an internal Kernel error. Ordinary duplicate
// registrations are returned as conflicts by registerSpec.
func safeRegisterSpec(spec *RunnerSpec) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xerr.Internal(fmt.Sprintf("register runner %q", spec.ID), errors.New(fmt.Sprint(r)))
		}
	}()
	return registerSpec(spec)
}

// ParseRunner parses one @lang.runner or @lang.define block using native NFlow
// key/value and list syntax.
func ParseRunner(src string) (*RunnerSpec, error) {
	header, block, err := runnerBlock(src)
	if err != nil {
		return nil, err
	}

	var spec RunnerSpec
	body := strings.TrimSpace(block[1 : len(block)-1])
	if err := parseRunnerFields(body, &spec); err != nil {
		return nil, err
	}

	if header.ID != "" {
		if spec.ID != "" && spec.ID != header.ID {
			return nil, xerr.BadRequest(fmt.Sprintf("@lang.runner header ID %q does not match profile ID %q", header.ID, spec.ID))
		}
		spec.ID = header.ID
	}
	if header.Version != "" {
		if spec.Version != "" && spec.Version != header.Version {
			return nil, xerr.BadRequest(fmt.Sprintf("@lang.runner header version %q does not match profile version %q", header.Version, spec.Version))
		}
		spec.Version = header.Version
	}
	if err := validateParsedRunner(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func validateParsedRunner(spec *RunnerSpec) error {
	if spec.ID == "" {
		return xerr.BadRequest(`runner: "id" is required`)
	}
	if spec.Ext == "" {
		return xerr.Validation(fmt.Sprintf("@lang.runner %s: ext is required", spec.ID))
	}
	if len(spec.Find) == 0 {
		return xerr.Validation(fmt.Sprintf("@lang.runner %s: find is required", spec.ID))
	}
	if err := resolveTemplateFile(spec); err != nil {
		return err
	}
	if spec.FilePathMode != "" && spec.FilePathMode != "wsl" {
		return xerr.Validation(fmt.Sprintf("runner %s: unsupported file_path_mode %q", spec.ID, spec.FilePathMode))
	}
	if err := validateFileTemplate(spec); err != nil {
		return err
	}
	if spec.Deps != "" || spec.DepsMarker != "" {
		parts := strings.Fields(spec.Deps)
		if len(parts) < 2 || spec.DepsMarker == "" || !strings.Contains(strings.Join(parts[1:], " "), "{file}") {
			return xerr.Validation(fmt.Sprintf("@lang.runner %s: deps requires a command, deps_marker, and {file} argument", spec.ID))
		}
	}
	return nil
}

type runnerHeader struct {
	ID      string
	Version string
}

func runnerBlock(src string) (runnerHeader, string, error) {
	var header runnerHeader
	blockStart := -1
	for _, line := range lineOffsets(src) {
		trimmed := strings.TrimSpace(line.text)
		if !strings.HasPrefix(trimmed, "@lang.runner") && !strings.HasPrefix(trimmed, "@lang.define") {
			continue
		}
		parsedHeader, err := parseRunnerHeader(trimmed)
		if err != nil {
			return runnerHeader{}, "", err
		}
		header = parsedHeader
		openInLine := strings.IndexByte(line.text, '{')
		if openInLine >= 0 {
			blockStart = line.offset + openInLine
		} else {
			searchFrom := line.offset + len(line.text)
			if next := strings.IndexByte(src[searchFrom:], '{'); next >= 0 {
				blockStart = searchFrom + next
			}
		}
		break
	}
	if blockStart < 0 {
		return runnerHeader{}, "", xerr.BadRequest("runner profile must contain an @lang.runner or @lang.define block")
	}
	blockEnd, err := matchingBrace(src, blockStart)
	if err != nil {
		return runnerHeader{}, "", err
	}
	return header, src[blockStart : blockEnd+1], nil
}

type sourceLine struct {
	offset int
	text   string
}

func lineOffsets(src string) []sourceLine {
	lines := strings.Split(src, "\n")
	out := make([]sourceLine, 0, len(lines))
	offset := 0
	for _, line := range lines {
		out = append(out, sourceLine{offset: offset, text: line})
		offset += len(line) + 1
	}
	return out
}

func parseRunnerHeader(line string) (runnerHeader, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 || len(fields) > 4 {
		return runnerHeader{}, xerr.BadRequest("@lang.runner requires an ID and optional version")
	}
	parseToken := func(raw string) (string, error) {
		if raw == "{" {
			return "", nil
		}
		if strings.HasPrefix(raw, `"`) {
			value, err := strconv.Unquote(raw)
			if err != nil {
				return "", xerr.BadRequest("@lang.runner header contains an invalid quoted value", err)
			}
			return value, nil
		}
		return strings.Trim(raw, `"'`), nil
	}
	id, err := parseToken(fields[1])
	if err != nil {
		return runnerHeader{}, err
	}
	if id == "" {
		return runnerHeader{}, xerr.BadRequest("@lang.runner ID cannot be empty")
	}
	header := runnerHeader{ID: id}
	if len(fields) >= 3 && fields[2] != "{" {
		header.Version, err = parseToken(fields[2])
		if err != nil {
			return runnerHeader{}, err
		}
	}
	return header, nil
}

func matchingBrace(src string, open int) (int, error) {
	depth, inString, escaped := 0, false, false
	for i := open; i < len(src); i++ {
		ch := src[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, xerr.BadRequest("runner profile has an unterminated block")
}

func parseRunnerFields(body string, spec *RunnerSpec) error {
	seen := make(map[string]struct{})
	for lineNumber, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ","))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		key, rawValue, ok := splitRunnerField(line)
		if !ok {
			return xerr.BadRequest(fmt.Sprintf("runner profile line %d: expected key: value", lineNumber+1))
		}
		if _, dup := seen[key]; dup {
			return xerr.BadRequest(fmt.Sprintf("runner profile: duplicate field %q", key))
		}
		seen[key] = struct{}{}

		if err := parseRunnerField(key, rawValue, spec); err != nil {
			return err
		}
	}
	return nil
}

func parseRunnerField(key, rawValue string, spec *RunnerSpec) error {
	switch key {
	case "id", "version", "ext", "stdin", "stdout", "file_template", "file_template_file", "file_path_mode", "deps", "deps_marker":
		value, err := parseRunnerScalar(rawValue)
		if err != nil {
			return err
		}
		setRunnerScalarField(spec, key, value)
	case "aliases", "find", "install_windows", "install_darwin", "install_linux":
		values, err := parseRunnerList(rawValue)
		if err != nil {
			return err
		}
		setRunnerListField(spec, key, values)
	default:
		return xerr.BadRequest(fmt.Sprintf("runner profile: unknown field %q", key))
	}
	return nil
}

func setRunnerScalarField(spec *RunnerSpec, key, value string) {
	switch key {
	case "id":
		spec.ID = value
	case "version":
		spec.Version = value
	case "ext":
		spec.Ext = value
	case "stdin":
		spec.Stdin = value
	case "stdout":
		spec.Stdout = value
	case "file_template":
		spec.FileTemplate = value
	case "file_template_file":
		spec.FileTemplateFile = value
	case "file_path_mode":
		spec.FilePathMode = value
	case "deps":
		spec.Deps = value
	case "deps_marker":
		spec.DepsMarker = value
	}
}

func setRunnerListField(spec *RunnerSpec, key string, values []string) {
	switch key {
	case "aliases":
		spec.Aliases = values
	case "find":
		spec.Find = values
	case "install_windows":
		spec.Install.Windows = values
	case "install_darwin":
		spec.Install.Darwin = values
	case "install_linux":
		spec.Install.Linux = values
	}
}

func splitRunnerField(line string) (key, value string, ok bool) {
	inQuote, escaped := byte(0), false
	for i := range len(line) {
		ch := line[i]
		if inQuote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inQuote = ch
			continue
		}
		if ch == ':' {
			key = strings.TrimSpace(line[:i])
			value = strings.TrimSpace(line[i+1:])
			return key, value, key != "" && value != ""
		}
	}
	return "", "", false
}

func parseRunnerScalar(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", xerr.BadRequest("runner profile: value cannot be empty")
	}
	if strings.HasPrefix(raw, `"`) {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", xerr.BadRequest("runner profile: invalid quoted string", err)
		}
		return value, nil
	}
	if len(raw) >= 2 && (raw[0] == '\'' && raw[len(raw)-1] == '\'' || raw[0] == '`' && raw[len(raw)-1] == '`') {
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}

func parseRunnerList(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, xerr.BadRequest("runner profile: expected a list in [value, ...] form")
	}
	content := strings.TrimSpace(raw[1 : len(raw)-1])
	if content == "" {
		return nil, nil
	}
	parts := splitRunnerList(content)
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value, err := parseRunnerScalar(part)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func splitRunnerList(content string) []string {
	var parts []string
	start, quote, escaped := 0, byte(0), false
	for i := range len(content) {
		ch := content[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			quote = ch
			continue
		}
		if ch == ',' {
			parts = append(parts, strings.TrimSpace(content[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(content[start:]))
	return parts
}

func resolveTemplateFile(spec *RunnerSpec) error {
	if spec.FileTemplateFile == "" {
		return nil
	}
	if spec.FileTemplate != "" {
		return xerr.Validation(fmt.Sprintf("runner %s: set only one of file_template and file_template_file", spec.ID))
	}
	if !fs.ValidPath(spec.FileTemplateFile) || !strings.HasPrefix(spec.FileTemplateFile, "templates/") {
		return xerr.Validation(fmt.Sprintf("runner %s: file_template_file must be inside templates/", spec.ID))
	}
	data, err := runnersFS.ReadFile(path.Join("runners", spec.FileTemplateFile))
	if err != nil {
		return xerr.Validation(fmt.Sprintf("runner %s: read file template %q", spec.ID, spec.FileTemplateFile), err)
	}
	spec.FileTemplate = string(data)
	return nil
}

func validateFileTemplate(spec *RunnerSpec) error {
	if spec.FileTemplate == "" {
		return nil
	}
	if strings.TrimSpace(spec.FileTemplate) == "" {
		return xerr.Validation(fmt.Sprintf("runner %s: file_template cannot be empty", spec.ID))
	}
	if (spec.Stdin != "" && spec.Stdin != "json") || (spec.Stdout != "" && spec.Stdout != "json") {
		return xerr.Validation(fmt.Sprintf("runner %s: file_template requires stdin and stdout codecs to be json", spec.ID))
	}
	return nil
}

// registerSpec validates and converts a RunnerSpec to a Language, then
// registers it with its aliases. Validate every collision first so a user
// supplied duplicate cannot leave a partially registered spec.
func registerSpec(spec *RunnerSpec) error {
	if err := validateRegistrationSpec(spec); err != nil {
		return err
	}
	spec = cloneRunnerSpec(spec)
	key := registryKey(spec.ID, spec.Version)
	if _, dup := Registry[key]; dup {
		return xerr.Conflict(fmt.Sprintf("lang: language %s already registered", key))
	}
	if err := validateRunnerAliases(spec, key); err != nil {
		return err
	}

	enc, err := spec.Encode()
	if err != nil {
		return err
	}
	dec, err := spec.Decode()
	if err != nil {
		return err
	}
	language := Language{
		ID:            spec.ID,
		Ext:           spec.Ext,
		Encode:        enc,
		Decode:        dec,
		spec:          spec,
		SelectContext: specContextFunc(spec),
		install:       spec.Install,
	}
	Register(language)
	for _, alias := range spec.Aliases {
		registerAlias(alias, spec.ID, spec.Version)
	}
	return nil
}

func validateRegistrationSpec(spec *RunnerSpec) error {
	if spec == nil || spec.ID == "" {
		return xerr.Validation("lang: runner ID is required")
	}
	if spec.Ext == "" {
		return xerr.Validation(fmt.Sprintf("lang.%s: runner extension is required", spec.ID))
	}
	if len(spec.Find) == 0 {
		return xerr.Validation(fmt.Sprintf("lang.%s: at least one interpreter command is required", spec.ID))
	}
	if spec.Deps != "" || spec.DepsMarker != "" {
		parts := strings.Fields(spec.Deps)
		if len(parts) < 2 || spec.DepsMarker == "" || !strings.Contains(strings.Join(parts[1:], " "), "{file}") {
			return xerr.Validation(fmt.Sprintf("lang.%s: deps requires a command, deps_marker, and {file} argument", spec.ID))
		}
	}
	if spec.FilePathMode != "" && spec.FilePathMode != "wsl" {
		return xerr.Validation(fmt.Sprintf("lang.%s: unsupported file_path_mode %q", spec.ID, spec.FilePathMode))
	}
	return validateFileTemplate(spec)
}

func validateRunnerAliases(spec *RunnerSpec, key string) error {
	seen := make(map[string]struct{}, len(spec.Aliases))
	for _, alias := range spec.Aliases {
		if alias == "" {
			continue
		}
		if _, dup := seen[alias]; dup {
			return xerr.Conflict(fmt.Sprintf("lang: alias @%s is repeated in %s", alias, key))
		}
		seen[alias] = struct{}{}
		for _, registered := range aliases {
			if registered.Tag == alias {
				return xerr.Conflict(fmt.Sprintf("lang: alias @%s already registered for %s", alias, registryKey(registered.ID, registered.Version)))
			}
		}
	}
	return nil
}

// RegisterRunnerSpec validates and registers a runner profile and all of its
// aliases. Duplicate IDs or aliases return a Kernel Conflict error without
// partial mutation.
func RegisterRunnerSpec(spec *RunnerSpec) error { return registerSpec(spec) }

func cloneRunnerSpec(spec *RunnerSpec) *RunnerSpec {
	if spec == nil {
		return nil
	}
	clone := *spec
	clone.Aliases = append([]string(nil), spec.Aliases...)
	clone.Find = append([]string(nil), spec.Find...)
	clone.Install.Windows = append([]string(nil), spec.Install.Windows...)
	clone.Install.Darwin = append([]string(nil), spec.Install.Darwin...)
	clone.Install.Linux = append([]string(nil), spec.Install.Linux...)
	return &clone
}

func specContextFunc(spec *RunnerSpec) func(context.Context, string) (string, []string, error) {
	return func(ctx context.Context, _ string) (string, []string, error) {
		return spec.FindCmdContext(ctx)
	}
}
