// Package query embeds xo/usql to provide sequa's SQL client: a one-shot
// command runner and an interactive REPL with psql-style backslash commands.
package query

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/xo/usql/handler"
	"github.com/xo/usql/metacmd"
	"github.com/xo/usql/rline"
	"github.com/xo/usql/stmt"

	_ "github.com/xo/usql/drivers/postgres" // register postgres with usql/dburl
)

// Run connects to dsn. When command is non-empty it executes that single SQL
// statement and returns; otherwise it starts the interactive usql REPL.
func Run(ctx context.Context, dsn, command string) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("current user: %w", err)
	}
	wd, _ := os.Getwd()

	interactive := command == ""
	l, err := rline.New(interactive, false, !interactive, "", "")
	if err != nil {
		return fmt.Errorf("init line editor: %w", err)
	}

	h := handler.New(l, u, wd, nil, true)
	if err := h.Open(ctx, dsn); err != nil {
		return fmt.Errorf("connect to %s: %w", redact(dsn), err)
	}

	if command != "" {
		// The prefix tells usql whether the statement returns rows (SELECT) or
		// is an exec (INSERT/UPDATE/…); without it a SELECT renders as "EXEC N".
		prefix := stmt.FindPrefix(command, true, true, true)
		return h.Execute(ctx, os.Stdout, metacmd.Option{}, prefix, command, false)
	}
	return h.Run()
}

// redact returns the DSN scheme only, never the credentials.
func redact(dsn string) string {
	if i := strings.Index(dsn, "://"); i > 0 {
		return dsn[:i] + "://…"
	}
	return "database"
}
