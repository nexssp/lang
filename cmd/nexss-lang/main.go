// Command nexss-lang inspects supported source declarations and generates
// language-neutral contracts and NFlow schema starters.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nexssp/lang/contract"
	"github.com/nexssp/lang/languages/python"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "inspect":
		return inspect(args[1:], stdout, stderr)
	case "generate":
		return generate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func inspect(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	className := flags.String("class", "", "inspect only this dataclass or TypedDict")
	pythonCommand := flags.String("python", "python3", "Python interpreter used for safe AST parsing")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nexss-lang inspect [--class NAME] [--python EXECUTABLE] FILE.py")
		return 2
	}
	contracts, err := inspectFile(flags.Arg(0), *className, *pythonCommand)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(contracts) == 0 {
		fmt.Fprintln(stderr, "no supported dataclass or TypedDict declarations found")
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(contracts); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func generate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	className := flags.String("class", "", "generate only this dataclass or TypedDict")
	pythonCommand := flags.String("python", "python3", "Python interpreter used for safe AST parsing")
	outputPath := flags.String("output", "", "write the generated NFlow file to this path instead of stdout")
	flags.StringVar(outputPath, "o", "", "alias for --output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nexss-lang generate [--class NAME] [--python EXECUTABLE] [--output FILE.nflow] FILE.py")
		return 2
	}
	contracts, err := inspectFile(flags.Arg(0), *className, *pythonCommand)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(contracts) == 0 {
		fmt.Fprintln(stderr, "no supported dataclass or TypedDict declarations found")
		return 1
	}
	nflow, err := contract.RenderNFlow(contracts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *outputPath == "" {
		if _, err := io.WriteString(stdout, nflow); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if err := writeNewFile(*outputPath, nflow); err != nil {
		fmt.Fprintf(stderr, "write %s: %v\n", *outputPath, err)
		return 1
	}
	fmt.Fprintf(stderr, "wrote %s\n", *outputPath)
	return 0
}

func writeNewFile(path, contents string) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	keepFile := false
	defer func() {
		if !keepFile {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()

	if _, err := file.WriteString(contents); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	keepFile = true
	return nil
}

func inspectFile(path, className, pythonCommand string) ([]contract.Contract, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	contracts, err := python.Inspect(context.Background(), string(source), className, python.InspectOptions{PythonCommand: pythonCommand})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return contracts, nil
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  nexss-lang inspect [--class NAME] [--python EXECUTABLE] FILE.py")
	fmt.Fprintln(writer, "  nexss-lang generate [--class NAME] [--python EXECUTABLE] [--output FILE.nflow] FILE.py")
	fmt.Fprintln(writer, "\nCurrent source support: Python dataclasses and TypedDict declarations.")
	fmt.Fprintln(writer, "Python is used only to parse the source AST; the input module is never executed.")
}
