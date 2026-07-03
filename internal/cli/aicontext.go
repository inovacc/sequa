package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// sequaOverview is the one-paragraph description emitted at the top of the AI
// context document.
const sequaOverview = "sequa is one Go tool for PostgreSQL: SQL migrations (`migrate`), an " +
	"interactive SQL client/REPL (`query`), type-safe Go codegen driven by the " +
	"migrations (`generate`), and schema-drift verification (`verify`) — plus an " +
	"embeddable migration library (`pkg/sequa`). Migrations are the single source of truth."

// newAicontextCmd builds the `aicontext` command, which emits a structured
// reference of every command and flag for AI tools to consume as context.
func newAicontextCmd() *cobra.Command {
	var (
		jsonOut bool
		compact bool
	)
	cmd := &cobra.Command{
		Use:   "aicontext",
		Short: "Generate AI context documentation",
		Long: "Generate structured documentation about sequa for use by AI tools.\n" +
			"\n" +
			"Outputs a Markdown (or JSON) reference of every command, flag, and usage\n" +
			"pattern that AI assistants can consume as context.\n" +
			"\n" +
			"Examples:\n" +
			"  sequa aicontext             # Markdown output (default)\n" +
			"  sequa aicontext --json      # structured JSON\n" +
			"  sequa aicontext --compact   # shorter output",
		RunE: func(c *cobra.Command, _ []string) error {
			root := c.Root()
			switch {
			case jsonOut:
				return printAIContextJSON(c, root)
			case compact:
				return printAIContextCompact(c, root)
			default:
				return printAIContextMarkdown(c, root)
			}
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	cmd.Flags().BoolVar(&compact, "compact", false, "shorter output")
	return cmd
}

// aiCommandInfo is the JSON shape of a command for `aicontext --json`.
type aiCommandInfo struct {
	Name        string          `json:"name"`
	Usage       string          `json:"usage"`
	Description string          `json:"description"`
	Flags       []aiFlagInfo    `json:"flags,omitempty"`
	Subcommands []aiCommandInfo `json:"subcommands,omitempty"`
}

// aiFlagInfo is the JSON shape of a flag for `aicontext --json`.
type aiFlagInfo struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
	Global      bool   `json:"global,omitempty"`
}

// aiContextDoc is the top-level JSON document for `aicontext --json`.
type aiContextDoc struct {
	Tool        string          `json:"tool"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	GlobalFlags []aiFlagInfo    `json:"global_flags,omitempty"`
	Commands    []aiCommandInfo `json:"commands"`
}

func printAIContextMarkdown(cmd, root *cobra.Command) error {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "# %s — AI Context\n\n", root.Name())
	_, _ = b.WriteString("## Overview\n\n")
	_, _ = fmt.Fprintf(&b, "%s\n\n", sequaOverview)

	if gf := collectPersistentFlags(root); len(gf) > 0 {
		_, _ = b.WriteString("## Global Flags\n\n")
		_, _ = b.WriteString("These flags apply to all commands:\n\n")
		for _, f := range gf {
			writeFlagBullet(&b, f)
		}
		_, _ = b.WriteString("\n")
	}

	_, _ = b.WriteString("## Commands\n\n")
	aiWriteCommandMarkdown(&b, root.Commands(), "")

	writeCategories(&b)
	writeStructure(&b)

	_, _ = fmt.Fprint(cmd.OutOrStdout(), b.String())
	return nil
}

// writeCategories lists sequa's commands grouped by purpose. The grouping is a
// static, ordered slice so the output is deterministic.
func writeCategories(b *strings.Builder) {
	categories := []struct {
		name string
		cmds []string
	}{
		{"Migrations", []string{"init", "migrate"}},
		{"Codegen", []string{"generate"}},
		{"Query", []string{"query"}},
		{"Schema", []string{"verify"}},
		{"Tooling", []string{"cmdtree", "aicontext"}},
	}
	_, _ = b.WriteString("## Command Categories\n\n")
	for _, cat := range categories {
		_, _ = fmt.Fprintf(b, "### %s\n\n", cat.name)
		for _, name := range cat.cmds {
			_, _ = fmt.Fprintf(b, "- `%s`\n", name)
		}
		_, _ = b.WriteString("\n")
	}
}

// writeStructure documents sequa's package layout.
func writeStructure(b *strings.Builder) {
	structure := []string{
		"cmd/sequa/        # CLI entrypoint",
		"internal/cli/     # Cobra commands",
		"internal/config/  # dir/DSN resolution",
		"internal/db/      # connection layer",
		"internal/migrate/ # migration engine + goose-style UX",
		"internal/query/   # embedded SQL client/REPL",
		"internal/codegen/ # schema catalog + Go codegen",
		"pkg/sequa/        # public embeddable API",
	}
	_, _ = b.WriteString("## Project Structure\n\n```\n")
	for _, s := range structure {
		_, _ = fmt.Fprintf(b, "%s\n", s)
	}
	_, _ = b.WriteString("```\n")
}

func printAIContextCompact(cmd, root *cobra.Command) error {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "# %s — %s\n\n", root.Name(), root.Short)

	persistent := collectPersistentFlags(root)
	globalParts := make([]string, 0, len(persistent))
	for _, f := range persistent {
		globalParts = append(globalParts, fmt.Sprintf("`--%s` %s", f.Name, f.Description))
	}
	if len(globalParts) > 0 {
		_, _ = b.WriteString("**Global:** ")
		_, _ = b.WriteString(strings.Join(globalParts, ", "))
		_, _ = b.WriteString("\n\n")
	}

	for _, c := range visibleCommands(root.Commands()) {
		aiWriteCompactCommand(&b, root.Name(), c, "")
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), b.String())
	return nil
}

func printAIContextJSON(cmd, root *cobra.Command) error {
	doc := aiContextDoc{
		Tool:        root.Name(),
		Version:     root.Version,
		Description: root.Short,
	}
	for _, f := range collectPersistentFlags(root) {
		doc.GlobalFlags = append(doc.GlobalFlags, aiFlagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Type,
			Default:     f.Default,
			Description: f.Description,
			Global:      true,
		})
	}
	for _, c := range visibleCommands(root.Commands()) {
		doc.Commands = append(doc.Commands, aiBuildCommandInfo(c))
	}
	return encodeJSON(cmd, doc)
}

func aiBuildCommandInfo(cmd *cobra.Command) aiCommandInfo {
	info := aiCommandInfo{
		Name:        cmd.Name(),
		Usage:       cmd.UseLine(),
		Description: cmd.Short,
	}
	for _, f := range collectFlags(cmd) {
		info.Flags = append(info.Flags, aiFlagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Type,
			Default:     f.Default,
			Description: f.Description,
		})
	}
	for _, sub := range visibleCommands(cmd.Commands()) {
		info.Subcommands = append(info.Subcommands, aiBuildCommandInfo(sub))
	}
	return info
}

func aiWriteCommandMarkdown(b *strings.Builder, commands []*cobra.Command, prefix string) {
	for _, c := range visibleCommands(commands) {
		heading := "###"
		name := c.Name()
		if prefix != "" {
			heading = "####"
			name = prefix + " " + name
		}
		_, _ = fmt.Fprintf(b, "%s %s\n\n", heading, name)
		_, _ = fmt.Fprintf(b, "Usage: `%s`\n\n", c.UseLine())
		_, _ = fmt.Fprintf(b, "%s\n\n", c.Short)

		writeFlagList(b, c)

		if len(c.Commands()) > 0 {
			aiWriteCommandMarkdown(b, c.Commands(), c.Name())
		}
	}
}

func aiWriteCompactCommand(b *strings.Builder, tool string, cmd *cobra.Command, prefix string) {
	name := cmd.Name()
	if prefix != "" {
		name = prefix + " " + name
	}
	_, _ = fmt.Fprintf(b, "- `%s %s` - %s", tool, name, cmd.Short)

	local := collectFlags(cmd)
	flagParts := make([]string, 0, len(local))
	for _, f := range local {
		flagParts = append(flagParts, fmt.Sprintf("`--%s`", f.Name))
	}
	if len(flagParts) > 0 {
		_, _ = fmt.Fprintf(b, " [%s]", strings.Join(flagParts, ", "))
	}
	_, _ = b.WriteString("\n")

	for _, sub := range visibleCommands(cmd.Commands()) {
		aiWriteCompactCommand(b, tool, sub, name)
	}
}

// writeFlagBullet renders one global flag as a Markdown list item.
func writeFlagBullet(b *strings.Builder, f flagDetail) {
	_, _ = fmt.Fprintf(b, "- `--%s`", f.Name)
	if f.Shorthand != "" {
		_, _ = fmt.Fprintf(b, ", `-%s`", f.Shorthand)
	}
	_, _ = fmt.Fprintf(b, " - %s", f.Description)
	if f.Default != "" && f.Default != "false" {
		_, _ = fmt.Fprintf(b, " (default: %s)", f.Default)
	}
	_, _ = b.WriteString("\n")
}

// writeFlagList renders a command's local flags as a Markdown list.
func writeFlagList(b *strings.Builder, cmd *cobra.Command) {
	flags := collectFlags(cmd)
	if len(flags) == 0 {
		return
	}
	_, _ = b.WriteString("Flags:\n")
	for _, f := range flags {
		_, _ = fmt.Fprintf(b, "- `--%s`", f.Name)
		if f.Shorthand != "" {
			_, _ = fmt.Fprintf(b, ", `-%s`", f.Shorthand)
		}
		_, _ = fmt.Fprintf(b, " - %s", f.Description)
		if f.Default != "" && f.Default != "false" && f.Default != "0" {
			_, _ = fmt.Fprintf(b, " (default: %s)", f.Default)
		}
		_, _ = b.WriteString("\n")
	}
	_, _ = b.WriteString("\n")
}
