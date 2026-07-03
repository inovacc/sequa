package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ASCII tree drawing characters — fixed width so the tree aligns on every
// terminal, and column at which trailing "# description" comments start.
const (
	treeMiddle = "+-- "
	treeLast   = "\\-- "
	treeIndent = "|   "
	treeSpace  = "    "
	maxDescLen = 40
	commentCol = 45
)

// flagDetail is a flattened view of a single cobra/pflag flag for rendering.
type flagDetail struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

// commandDetail is the JSON shape of a command and its subtree, emitted by
// `cmdtree --json`.
type commandDetail struct {
	Name        string          `json:"name"`
	Use         string          `json:"use"`
	Short       string          `json:"short"`
	Long        string          `json:"long,omitempty"`
	Flags       []flagDetail    `json:"flags,omitempty"`
	GlobalFlags []flagDetail    `json:"global_flags,omitempty"`
	Subcommands []commandDetail `json:"commands,omitempty"`
}

// newCmdtreeCmd builds the `cmdtree` command, which visualizes the whole
// command tree — as a compact ASCII tree, a detailed per-command listing, or
// JSON — for humans and tooling.
func newCmdtreeCmd() *cobra.Command {
	var (
		full    bool
		jsonOut bool
		command string
	)
	cmd := &cobra.Command{
		Use:   "cmdtree",
		Short: "Display a tree of all commands",
		Long:  "Display a tree visualization of every command with its description, or the full details of a single command.",
		RunE: func(c *cobra.Command, _ []string) error {
			root := c.Root()
			switch {
			case jsonOut && command != "":
				return printSingleCommandJSON(c, root, command)
			case jsonOut:
				return encodeJSON(c, buildCommandDetail(root))
			case command != "":
				return printSingleCommandText(c, root, command)
			}

			var tree bytes.Buffer
			_, _ = tree.WriteString("# Command Tree\n\n```\n")
			if full {
				_, _ = tree.Write(buildVerboseTree(root))
			} else {
				_, _ = tree.Write(buildTree(root))
			}
			_, _ = tree.WriteString("```\n")
			_, _ = fmt.Fprintln(c.OutOrStdout(), tree.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "show full per-command details (usage, description, flags)")
	cmd.Flags().StringVarP(&command, "command", "c", "", "show details for a single command only")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	return cmd
}

// describe returns a command's Long description, falling back to Short.
func describe(cmd *cobra.Command) string {
	if cmd.Long != "" {
		return cmd.Long
	}
	return cmd.Short
}

// shortDesc returns a command's Short description, falling back to Long.
func shortDesc(cmd *cobra.Command) string {
	if cmd.Short != "" {
		return cmd.Short
	}
	return cmd.Long
}

// visibleCommands drops hidden commands, preserving order.
func visibleCommands(commands []*cobra.Command) []*cobra.Command {
	out := make([]*cobra.Command, 0, len(commands))
	for _, c := range commands {
		if c.Hidden {
			continue
		}
		out = append(out, c)
	}
	return out
}

func buildTree(root *cobra.Command) []byte {
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "%s\n", root.Use)
	writeGlobalFlags(&buf, root)
	printCommands(&buf, root.Commands(), "")
	return buf.Bytes()
}

func buildVerboseTree(root *cobra.Command) []byte {
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "%s\n", root.Use)
	writeGlobalFlags(&buf, root)
	printVerboseCommands(&buf, root.Commands(), "")
	return buf.Bytes()
}

// writeGlobalFlags renders the root's persistent flags once, at the top.
func writeGlobalFlags(w io.Writer, root *cobra.Command) {
	globalFlags := collectPersistentFlags(root)
	if len(globalFlags) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%s\n", treeIndent)
	_, _ = fmt.Fprintf(w, "%sGlobal Flags:\n", treeIndent)
	for _, f := range globalFlags {
		printFlagDetail(w, treeIndent+"  ", f)
	}
	_, _ = fmt.Fprintf(w, "%s\n", treeIndent)
}

func printCommands(w io.Writer, commands []*cobra.Command, prefix string) {
	visible := visibleCommands(commands)
	for i, c := range visible {
		isLast := i == len(visible)-1
		connector := treeMiddle
		if isLast {
			connector = treeLast
		}
		desc := shortDesc(c)
		if len(desc) > maxDescLen {
			desc = desc[:maxDescLen-3] + "..."
		}
		cmdPart := prefix + connector + c.Name()
		padding := max(commentCol-len(cmdPart), 2)
		_, _ = fmt.Fprintf(w, "%s%s# %s\n", cmdPart, strings.Repeat(" ", padding), desc)

		if len(c.Commands()) > 0 {
			newPrefix := prefix + treeIndent
			if isLast {
				newPrefix = prefix + treeSpace
			}
			printCommands(w, c.Commands(), newPrefix)
		}
	}
}

func printVerboseCommands(w io.Writer, commands []*cobra.Command, prefix string) {
	visible := visibleCommands(commands)
	for i, c := range visible {
		isLast := i == len(visible)-1
		connector := treeMiddle
		if isLast {
			connector = treeLast
		}
		_, _ = fmt.Fprintf(w, "%s%s%s\n", prefix, connector, c.Name())

		detailPrefix := prefix + treeIndent
		if isLast {
			detailPrefix = prefix + treeSpace
		}
		_, _ = fmt.Fprintf(w, "%sUsage: %s\n", detailPrefix, c.UseLine())
		if desc := describe(c); desc != "" {
			_, _ = fmt.Fprintf(w, "%sDescription: %s\n", detailPrefix, desc)
		}
		if flags := collectFlags(c); len(flags) > 0 {
			_, _ = fmt.Fprintf(w, "%s\n", detailPrefix)
			_, _ = fmt.Fprintf(w, "%sFlags:\n", detailPrefix)
			for _, f := range flags {
				printFlagDetail(w, detailPrefix+"  ", f)
			}
		}
		_, _ = fmt.Fprintf(w, "%s\n", detailPrefix)

		if len(c.Commands()) > 0 {
			printVerboseCommands(w, c.Commands(), detailPrefix)
		}
	}
}

// collectFlags returns a command's local (non-persistent) flags.
func collectFlags(cmd *cobra.Command) []flagDetail {
	var flags []flagDetail
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		// Persistent flags are shown once in the global-flags section.
		if cmd.PersistentFlags().Lookup(f.Name) != nil {
			return
		}
		flags = append(flags, flagFrom(f))
	})
	return flags
}

// collectPersistentFlags returns a command's persistent flags.
func collectPersistentFlags(cmd *cobra.Command) []flagDetail {
	var flags []flagDetail
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		flags = append(flags, flagFrom(f))
	})
	return flags
}

func flagFrom(f *pflag.Flag) flagDetail {
	return flagDetail{
		Name:        f.Name,
		Shorthand:   f.Shorthand,
		Type:        f.Value.Type(),
		Default:     f.DefValue,
		Description: f.Usage,
	}
}

func printFlagDetail(w io.Writer, prefix string, f flagDetail) {
	var flagStr string
	if f.Shorthand != "" {
		flagStr = fmt.Sprintf("-%s, --%s", f.Shorthand, f.Name)
	} else {
		flagStr = fmt.Sprintf("    --%s", f.Name)
	}
	if f.Type != "bool" {
		flagStr += " " + f.Type
	}
	padding := max(26-len(flagStr), 2)
	_, _ = fmt.Fprintf(w, "%s%s%s%s\n", prefix, flagStr, strings.Repeat(" ", padding), f.Description)
}

func printSingleCommandJSON(cmd, root *cobra.Command, name string) error {
	target := findCommand(root, name)
	if target == nil {
		return fmt.Errorf("command not found: %s", name)
	}
	return encodeJSON(cmd, buildCommandDetail(target))
}

func printSingleCommandText(cmd, root *cobra.Command, name string) error {
	target := findCommand(root, name)
	if target == nil {
		return fmt.Errorf("command not found: %s", name)
	}

	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "# %s\n\n", target.Name())
	_, _ = fmt.Fprintf(&buf, "Usage: %s\n\n", target.UseLine())
	if desc := describe(target); desc != "" {
		_, _ = fmt.Fprintf(&buf, "Description: %s\n\n", desc)
	}
	if flags := collectFlags(target); len(flags) > 0 {
		_, _ = buf.WriteString("Flags:\n")
		for _, f := range flags {
			printFlagDetail(&buf, "  ", f)
		}
		_, _ = buf.WriteString("\n")
	}
	if gf := collectPersistentFlags(target); len(gf) > 0 {
		_, _ = buf.WriteString("Global Flags:\n")
		for _, f := range gf {
			printFlagDetail(&buf, "  ", f)
		}
		_, _ = buf.WriteString("\n")
	}
	if subs := visibleCommands(target.Commands()); len(subs) > 0 {
		_, _ = buf.WriteString("Subcommands:\n")
		for _, sub := range subs {
			_, _ = fmt.Fprintf(&buf, "  %s - %s\n", sub.Name(), sub.Short)
		}
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), buf.String())
	return nil
}

// findCommand walks the tree for a command by name (depth-first).
func findCommand(root *cobra.Command, name string) *cobra.Command {
	if root.Name() == name {
		return root
	}
	for _, c := range root.Commands() {
		if found := findCommand(c, name); found != nil {
			return found
		}
	}
	return nil
}

func buildCommandDetail(cmd *cobra.Command) commandDetail {
	detail := commandDetail{
		Name:        cmd.Name(),
		Use:         cmd.UseLine(),
		Short:       cmd.Short,
		Long:        cmd.Long,
		Flags:       collectFlags(cmd),
		GlobalFlags: collectPersistentFlags(cmd),
	}
	for _, sub := range visibleCommands(cmd.Commands()) {
		detail.Subcommands = append(detail.Subcommands, buildCommandDetail(sub))
	}
	return detail
}

// encodeJSON writes v as indented JSON to the command's output stream.
func encodeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	return nil
}
