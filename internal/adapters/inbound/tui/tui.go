package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"neabrain/internal/observability"
)

// Run starts an interactive prompt that accepts CLI-style commands.
func Run(ctx context.Context, args []string, in io.Reader, out io.Writer, errOut io.Writer, executor func(context.Context, []string, io.Writer, io.Writer) int) int {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}

	if len(args) > 0 {
		if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
			writeUsage(out)
			return 0
		}
	}
	if executor == nil {
		fmt.Fprintln(errOut, "tui executor is required")
		return 1
	}

	fmt.Fprintln(out, "NeaBrain TUI. Commands match CLI. Type 'help' or 'exit'.")
	logger := observability.DefaultLogger()
	metrics := observability.DefaultMetrics()
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "nea> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if line == "help" {
			writeUsage(out)
			continue
		}
		parsed, err := parseArgs(line)
		if err != nil {
			fmt.Fprintln(errOut, err.Error())
			continue
		}
		if len(parsed) == 0 {
			continue
		}
		if parsed[0] == "tui" {
			fmt.Fprintln(errOut, "nested tui command is not supported")
			continue
		}
		logger.Info("tui command", map[string]any{"operation": parsed[0]})
		metrics.Inc("adapter.tui.command")
		_ = executor(ctx, parsed, out, errOut)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	return 0
}

func writeUsage(out io.Writer) {
	fmt.Fprintln(out, "Enter CLI commands at the prompt.")
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  observation create --content \"hello\"")
	fmt.Fprintln(out, "  search --query \"hello\"")
	fmt.Fprintln(out, "  session open --disclosure-level low")
	fmt.Fprintln(out, "  config show")
}

func parseArgs(input string) ([]string, error) {
	args := make([]string, 0)
	var current strings.Builder
	var quote byte
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteByte(ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' {
			flush()
			continue
		}
		current.WriteByte(ch)
	}
	if escaped {
		current.WriteByte('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return args, nil
}
