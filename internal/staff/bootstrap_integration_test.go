//go:build integration

package staff_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/staff"
	"github.com/aleogr/marketplace/internal/tenancy"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

// serving runs as the application role, the way the deployed service does,
// so row-level security is what is tested.
type serving struct {
	t    *testing.T
	pool *db.Pool
}

func (s serving) InTxFor(_ context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	return dbtest.Serving(s.t, s.pool, marketplaceID, fn)
}

func (s serving) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return dbtest.Serving(s.t, s.pool, "", fn)
}

// recording is an Auditor that remembers the entries it was given.
type recording struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (r *recording) Append(_ context.Context, _ pgx.Tx, e audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}

func (r *recording) find(action string) (audit.Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.Action == action {
			return e, true
		}
	}
	return audit.Entry{}, false
}

var (
	quiet = slog.New(slog.NewTextHandler(io.Discard, nil))
	cheap = identity.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}
)

const password = "correct horse battery staple"

// fresh is a database of the test's own, migrated, with the application role
// ready. The first run happens once per database, so no two tests may share
// one.
func fresh(t *testing.T) (serving, *identity.Service, *recording) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.Fresh(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	trail := &recording{}
	service := identity.NewService(serving{t, pool}, identity.NewHasher(cheap, 2),
		breached.Fake{Known: breached.Common}, trail, quiet)
	return serving{t, pool}, service, trail
}

// platform is a visit on the console's host.
func platform() identity.Visit {
	return identity.Visit{Platform: true, MarketplaceName: "Console", Language: "en-US",
		IP: "203.0.113.5", UserAgent: "test", BaseURL: "https://console.test"}
}

const token = "a-bootstrap-token-of-forty-eight-characters-0123"

// The first run: a wrong token is refused and changes nothing; the right one
// creates the owner, signs them in, records the token's SHA-256 as used, and
// closes the setup for good — the same token, or any other, is then refused
// as though the setup did not exist (F15 spec, D2).
func TestTheFirstRunCreatesTheOwnerOnce(t *testing.T) {
	database, service, trail := fresh(t)
	bootstrap := staff.NewBootstrap(database, service, trail, token)
	owners := staff.NewOwners(database)

	if open, err := bootstrap.Open(t.Context()); err != nil || !open {
		t.Fatalf("Open() before the owner = %v, %v; want true", open, err)
	}
	if _, err := bootstrap.Setup(t.Context(), platform(), "a-wrong-token", "Owner", "owner@example.test", password); !errors.Is(err, staff.ErrTokenWrong) {
		t.Fatalf("Setup with a wrong token = %v, want ErrTokenWrong", err)
	}
	if open, _ := bootstrap.Open(t.Context()); !open {
		t.Fatal("a wrong token closed the setup")
	}

	session, err := bootstrap.Setup(t.Context(), platform(), token, "Owner", "owner@example.test", password)
	if err != nil {
		t.Fatalf("Setup with the token = %v", err)
	}
	signedIn, err := service.AuthenticateStaff(t.Context(), session)
	if err != nil {
		t.Fatalf("the owner's session does not open: %v", err)
	}
	if signedIn.Account.Kind != identity.KindStaff || signedIn.Account.Email != "owner@example.test" {
		t.Fatalf("the owner is %+v", signedIn.Account)
	}
	for _, permission := range []staff.Permission{staff.ConsoleView, staff.ConsoleVersion} {
		if allowed, err := owners.Allows(t.Context(), signedIn.Account.ID, permission, staff.Platform); err != nil || !allowed {
			t.Errorf("the owner is allowed %s: %v, %v; want true", permission, allowed, err)
		}
	}

	var stored []byte
	var owner string
	if err := database.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT token_hash, owner_id::text FROM bootstrap`).Scan(&stored, &owner)
	}); err != nil {
		t.Fatal(err)
	}
	if want := sha256.Sum256([]byte(token)); !equal(stored, want[:]) || owner != signedIn.Account.ID {
		t.Fatalf("bootstrap holds %x for %s, want the token's SHA-256 for the owner", stored, owner)
	}
	created, found := trail.find("staff.owner_created")
	if !found || created.Actor.Kind != audit.Staff || created.Actor.ID != owner || created.Marketplace != "" {
		t.Fatalf("staff.owner_created recorded as %+v (found %v)", created, found)
	}

	if open, err := bootstrap.Open(t.Context()); err != nil || open {
		t.Fatalf("Open() after the owner = %v, %v; want false", open, err)
	}
	for _, again := range []string{token, "a-wrong-token"} {
		if _, err := bootstrap.Setup(t.Context(), platform(), again, "Second", "second@example.test", password); !errors.Is(err, staff.ErrSetupClosed) {
			t.Fatalf("Setup after the owner = %v, want ErrSetupClosed", err)
		}
	}
}

func equal(a, b []byte) bool { return hex.EncodeToString(a) == hex.EncodeToString(b) }

// Two first runs at once make one owner: the bootstrap holds one row, and
// the second waits for the first and finds the setup closed.
func TestTwoFirstRunsAtOnceMakeOneOwner(t *testing.T) {
	database, service, trail := fresh(t)
	bootstrap := staff.NewBootstrap(database, service, trail, token)

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = bootstrap.Setup(t.Context(), platform(), token, "Owner",
				[]string{"one@example.test", "two@example.test"}[i], password)
		}()
	}
	wg.Wait()

	succeeded, closed := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, staff.ErrSetupClosed):
			closed++
		default:
			t.Fatalf("a concurrent Setup = %v", err)
		}
	}
	if succeeded != 1 || closed != 1 {
		t.Fatalf("%d first runs succeeded and %d were closed, want one each", succeeded, closed)
	}
	var owners, accounts int
	if err := database.InTx(t.Context(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`SELECT count(*) FROM user_role u JOIN role r ON r.id = u.role_id WHERE r.owner`).Scan(&owners); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM account`).Scan(&accounts)
	}); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || accounts != 1 {
		t.Fatalf("%d owners and %d staff accounts, want one of each", owners, accounts)
	}
}

// Two first runs at once with the same e-mail race at the account, not at the
// bootstrap row: the loser's CreateStaff answers identity.ErrAddressTaken,
// which Setup must still answer as ErrSetupClosed once the winner's Open()
// shows an owner exists — the page already turns ErrSetupClosed into 404, and
// ErrAddressTaken would otherwise reach it as a 500 (F15 spec).
func TestTwoFirstRunsAtOnceWithTheSameAddressMakeOneOwner(t *testing.T) {
	database, service, trail := fresh(t)
	bootstrap := staff.NewBootstrap(database, service, trail, token)

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = bootstrap.Setup(t.Context(), platform(), token, "Owner", "owner@example.test", password)
		}()
	}
	wg.Wait()

	succeeded, closed := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, staff.ErrSetupClosed):
			closed++
		default:
			t.Fatalf("a concurrent Setup with the same address = %v, want nil or ErrSetupClosed", err)
		}
	}
	if succeeded != 1 || closed != 1 {
		t.Fatalf("%d first runs succeeded and %d were closed, want one each", succeeded, closed)
	}
	var owners, accounts int
	if err := database.InTx(t.Context(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(t.Context(),
			`SELECT count(*) FROM user_role u JOIN role r ON r.id = u.role_id WHERE r.owner`).Scan(&owners); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM account`).Scan(&accounts)
	}); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || accounts != 1 {
		t.Fatalf("%d owners and %d staff accounts, want one of each", owners, accounts)
	}
}

// A deployment given no token has no first run at all.
func TestWithoutATokenThereIsNoFirstRun(t *testing.T) {
	database, service, trail := fresh(t)
	bootstrap := staff.NewBootstrap(database, service, trail, "")
	if open, err := bootstrap.Open(t.Context()); err != nil || open {
		t.Fatalf("Open() with no token = %v, %v; want false", open, err)
	}
	if _, err := bootstrap.Setup(t.Context(), platform(), "", "Owner", "owner@example.test", password); !errors.Is(err, staff.ErrSetupClosed) {
		t.Fatalf("Setup with no token = %v, want ErrSetupClosed", err)
	}
}

// F13's rules hold for the owner: a breached password is refused, and the
// setup stays open.
func TestTheOwnersPasswordFollowsTheRules(t *testing.T) {
	database, service, trail := fresh(t)
	bootstrap := staff.NewBootstrap(database, service, trail, token)
	if _, err := bootstrap.Setup(t.Context(), platform(), token, "Owner", "owner@example.test", "password1234"); !errors.Is(err, identity.ErrPasswordBreached) {
		t.Fatalf("Setup with a breached password = %v, want ErrPasswordBreached", err)
	}
	if open, _ := bootstrap.Open(t.Context()); !open {
		t.Fatal("a refused password closed the setup")
	}
}

// Until roles exist (PR 2 of F15) the owner is the only staff member allowed
// anything: another staff account is allowed nothing.
func TestOnlyTheOwnerIsAllowed(t *testing.T) {
	database, service, trail := fresh(t)
	if _, err := staff.NewBootstrap(database, service, trail, token).
		Setup(t.Context(), platform(), token, "Owner", "owner@example.test", password); err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateStaff(t.Context(), platform(), "Colleague", "colleague@example.test", password, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.AuthenticateStaff(t.Context(), other)
	if err != nil {
		t.Fatal(err)
	}
	if allowed, err := staff.NewOwners(database).Allows(t.Context(), session.Account.ID, staff.ConsoleView, staff.Platform); err != nil || allowed {
		t.Fatalf("a staff member who is not the owner is allowed console.view: %v, %v", allowed, err)
	}
}

// The roles, their assignments and the bootstrap belong to the platform: a
// marketplace's transaction sees none of them.
func TestTheOwnersRowsAreThePlatforms(t *testing.T) {
	database, service, trail := fresh(t)
	if _, err := staff.NewBootstrap(database, service, trail, token).
		Setup(t.Context(), platform(), token, "Owner", "owner@example.test", password); err != nil {
		t.Fatal(err)
	}
	var marketplace string
	if err := database.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		applied, err := tenancy.Seed(t.Context(), tx, []tenancy.Spec{{Slug: "one", Name: "One", Market: "BR",
			RevenueModel: "commission", State: "active", DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"},
			Hosts: []string{"one.test"}}})
		if err == nil {
			marketplace = applied[0].ID
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"role", "user_role", "bootstrap"} {
		var seen int
		if err := database.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
			return tx.QueryRow(t.Context(), `SELECT count(*) FROM `+table).Scan(&seen)
		}); err != nil {
			t.Fatal(err)
		}
		if seen != 0 {
			t.Errorf("a marketplace sees %d rows of %s, want 0", seen, table)
		}
	}
}
