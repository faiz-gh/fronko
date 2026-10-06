package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/database"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
)

const adminUsage = `usage:
  fronko admin create --email you@example.com        add a platform admin
  fronko admin set-password --email you@example.com  replace an admin's password

The password is read from standard input.`

// adminCommand manages platform admins from the command line. There is no
// sign-up page for them, so this is the only way an admin account comes to be.
func adminCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(adminUsage)
	}
	fs := flag.NewFlagSet("admin "+args[0], flag.ContinueOnError)
	email := fs.String("email", "", "the admin's email address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(*email))
	if err != nil {
		return fmt.Errorf("--email: %w", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := repository.New(pool)

	password, err := readPassword()
	if err != nil {
		return err
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("password must be 12-72 characters")
	}
	// Only bcrypt is used here, so the JWT secret doesn't matter.
	hash, err := auth.NewService("").HashPassword(password)
	if err != nil {
		return err
	}

	switch args[0] {
	case "create":
		err = repo.CreatePlatformAdmin(ctx, &models.PlatformAdmin{Email: addr.Address, PasswordHash: hash})
		if errors.Is(err, repository.ErrConflict) {
			return fmt.Errorf("an admin with email %s already exists", addr.Address)
		}
	case "set-password":
		err = repo.SetPlatformAdminPassword(ctx, addr.Address, hash)
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("no admin with email %s", addr.Address)
		}
	default:
		return errors.New(adminUsage)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Done. %s can sign in at /admin/login.\n", addr.Address)
	return nil
}

// readPassword reads one line from standard input, hiding it when stdin is a
// terminal (stty is in the image via busybox).
func readPassword() (string, error) {
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Password (12-72 characters): ")
		if stty("-echo") == nil {
			defer func() {
				stty("echo")
				fmt.Fprintln(os.Stderr)
			}()
		}
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func stty(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
