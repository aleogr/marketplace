//go:build integration

package audit_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// unique returns a name no other test and no earlier run has used: the
// packages of one test run share a database in CI.
func unique(name string) string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		panic("cannot name a fixture: " + err.Error())
	}
	return name + "-" + hex.EncodeToString(raw)
}

// platform is the audit log over a real database, with a keeper of its own.
type platform struct {
	pool  *db.Pool
	trail *audit.Log
}

func assemble(t *testing.T) platform {
	t.Helper()

	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatalf("db.Open() = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(t.Context(), pool, settings, quiet()); err != nil {
		t.Fatalf("db.Migrate() = %v", err)
	}

	keeper, err := local.Generate()
	if err != nil {
		t.Fatalf("local.Generate() = %v", err)
	}
	return platform{pool: pool, trail: audit.NewLog(audit.NewKeys(keeper), quiet())}
}

// marketplace creates one and returns its identifier, with no host: the
// packages of one test run share a database, and the tenancy tests count hosts.
func (p platform) marketplace(t *testing.T, name string) string {
	t.Helper()

	var id string
	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			INSERT INTO marketplace (slug, name, market_code, revenue_model, default_language)
			VALUES ($1, $2, 'BR', 'commission', 'pt-BR')
			RETURNING id::text
		`, unique(name), name).Scan(&id)
	}); err != nil {
		t.Fatalf("cannot create the marketplace: %v", err)
	}
	return id
}

// append records one operation in the marketplace's own chain.
func (p platform) append(t *testing.T, marketplace string, entry audit.Entry) {
	t.Helper()

	entry.Marketplace = marketplace
	if err := p.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return p.trail.Append(t.Context(), tx, entry)
	}); err != nil {
		t.Fatalf("Append() = %v", err)
	}
}

// changed is an entry describing a parameter change by a person.
func changed(actor, subject string) audit.Entry {
	return audit.Entry{
		Actor:   audit.Actor{ID: actor, Kind: audit.Staff},
		Action:  "parameter.changed",
		Subject: audit.Subject{Kind: "parameter", ID: "commission.rate", Person: subject},
		From:    "203.0.113.7",
		Before:  json.RawMessage(`{"rate":"0.10"}`),
		After:   json.RawMessage(`{"rate":"0.12"}`),
	}
}

func person(t *testing.T) string {
	t.Helper()

	var id string
	if err := fmtScan(&id); err != nil {
		t.Fatalf("cannot name a person: %v", err)
	}
	return id
}

// fmtScan produces a uuid without reaching for a dependency the platform does
// not otherwise have: the database is right there.
func fmtScan(into *string) error {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(raw)
	*into = strings.Join([]string{
		hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32],
	}, "-")
	return nil
}

// TestTheLogRefusesToBeChanged is the delivery's first promise, and it is
// checked twice: the application role may not, and no role may
// (docs/roadmap.md, F12).
func TestTheLogRefusesToBeChanged(t *testing.T) {
	site := assemble(t)
	dbtest.AsApplication(t, site.pool)

	marketplace := site.marketplace(t, "audit-immutable")
	site.append(t, marketplace, changed(person(t), ""))

	t.Run("the application role is not allowed to", func(t *testing.T) {
		for name, statement := range map[string]string{
			"update": `UPDATE audit_log SET action = 'something else'`,
			"delete": `DELETE FROM audit_log`,
		} {
			t.Run(name, func(t *testing.T) {
				err := dbtest.Serving(t, site.pool, marketplace, func(tx pgx.Tx) error {
					_, err := tx.Exec(t.Context(), statement)
					return err
				})
				if err == nil {
					t.Fatalf("the application role was allowed to %s the audit log", name)
				}
				// Permissions, not the trigger: the role may not even try.
				if !strings.Contains(err.Error(), "permission denied") {
					t.Errorf("%s was refused by something other than permissions: %v", name, err)
				}
			})
		}
	})

	t.Run("and neither is the role that owns the table", func(t *testing.T) {
		for name, statement := range map[string]string{
			"update":   `UPDATE audit_log SET action = 'something else'`,
			"delete":   `DELETE FROM audit_log`,
			"truncate": `TRUNCATE audit_log`,
		} {
			t.Run(name, func(t *testing.T) {
				err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
					_, err := tx.Exec(t.Context(), statement)
					return err
				})
				if err == nil {
					t.Fatalf("the owner was allowed to %s the audit log", name)
				}
				if !strings.Contains(err.Error(), "append-only") {
					t.Errorf("%s was refused by something other than the trigger: %v", name, err)
				}
			})
		}
	})
}

// TestARecordAlteredOutOfBandIsDetected: the trigger is disabled the way an
// attacker with the owner's rights would disable it, the record is altered,
// and the verification says which one.
func TestARecordAlteredOutOfBandIsDetected(t *testing.T) {
	site := assemble(t)
	marketplace := site.marketplace(t, "audit-tamper")

	for range 3 {
		site.append(t, marketplace, changed(person(t), ""))
	}

	if broken := breaksIn(site.verify(t), marketplace); len(broken) != 0 {
		t.Fatalf("a chain nobody touched does not verify: %+v", broken)
	}

	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			`ALTER TABLE audit_log DISABLE TRIGGER audit_log_refuses_changes`); err != nil {
			return err
		}
		if _, err := tx.Exec(t.Context(), `
			UPDATE audit_log SET action = 'parameter.read'
			 WHERE marketplace_id = $1 AND sequence = 2
		`, marketplace); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(),
			`ALTER TABLE audit_log ENABLE TRIGGER audit_log_refuses_changes`)
		return err
	}); err != nil {
		t.Fatalf("cannot alter the record out of band: %v", err)
	}

	broken := breaksIn(site.verify(t), marketplace)
	if len(broken) == 0 {
		t.Fatal("a record was altered and the chain still verified")
	}

	var found bool
	for _, one := range broken {
		if one.Sequence == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("the verification did not name the altered record: %+v", broken)
	}
}

// verify walks every chain, which is what the scheduled job does.
func (site platform) verify(t *testing.T) audit.Report {
	t.Helper()

	report, err := site.trail.Verify(t.Context(), site.pool)
	if err != nil {
		t.Fatalf("Verify() = %v", err)
	}
	return report
}

// breaksIn narrows a report to one chain.
//
// Verification is deliberately global — the job walks everything — and the
// tests of one package share a database, so a test that asserted on the whole
// report would be asserting about the chain another test broke on purpose.
func breaksIn(report audit.Report, marketplace string) []audit.Break {
	var mine []audit.Break
	for _, broken := range report.Breaks {
		if broken.Marketplace == marketplace {
			mine = append(mine, broken)
		}
	}
	return mine
}

// TestDestroyingAKeyErasesTheContentAndLeavesTheChain is the delivery's
// objective: a deletion request is fulfilled without a hole in the log
// (docs/requirements.md, sections 21 and 18.3).
func TestDestroyingAKeyErasesTheContentAndLeavesTheChain(t *testing.T) {
	site := assemble(t)
	marketplace := site.marketplace(t, "audit-erasure")

	buyer := person(t)
	site.append(t, marketplace, changed(person(t), buyer))

	opened := site.open(t, marketplace, 1)
	switch {
	case opened.StateErased:
		t.Fatal("the state was unreadable before anything was destroyed")
	case string(opened.After) != `{"rate":"0.12"}`:
		t.Errorf("the state reads %q", opened.After)
	case opened.From != "203.0.113.7":
		t.Errorf("the origin reads %q", opened.From)
	}

	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return audit.NewKeys(nil).Destroy(t.Context(), tx, buyer)
	}); err != nil {
		t.Fatalf("Destroy() = %v", err)
	}

	erased := site.open(t, marketplace, 1)
	switch {
	case !erased.StateErased:
		t.Error("the state is still readable after the key was destroyed")
	case len(erased.After) != 0:
		t.Errorf("the state came back anyway: %q", erased.After)
	case erased.Action != "parameter.changed":
		t.Errorf("the record itself changed: action is %q", erased.Action)
	case erased.OriginErased:
		// The origin is the staff member's, and nobody asked for theirs.
		t.Error("the actor's own origin was erased with the subject's key")
	}

	if broken := breaksIn(site.verify(t), marketplace); len(broken) != 0 {
		t.Errorf("destroying a key broke the chain: %+v", broken)
	}
}

// open reads one record back.
func (site platform) open(t *testing.T, marketplace string, sequence int64) audit.Opened {
	t.Helper()

	var opened audit.Opened
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		var err error
		opened, err = site.trail.Open(t.Context(), tx, marketplace, sequence)
		return err
	}); err != nil {
		t.Fatalf("Open() = %v", err)
	}
	return opened
}

// TestNothingPersonalIsStoredInClearText reads the row as a person with the
// database open in front of them would.
func TestNothingPersonalIsStoredInClearText(t *testing.T) {
	site := assemble(t)
	marketplace := site.marketplace(t, "audit-clear-text")

	site.append(t, marketplace, changed(person(t), person(t)))

	var row string
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT audit_log::text FROM audit_log
			 WHERE marketplace_id = $1 AND sequence = 1
		`, marketplace).Scan(&row)
	}); err != nil {
		t.Fatalf("cannot read the row: %v", err)
	}

	// The origin address and the values that changed. The subject's identifier
	// is deliberately not among them: a parameter's name is how records about
	// that parameter are found, and it says nothing about a person
	// (docs/requirements.md, section 21).
	for _, secret := range []string{"203.0.113.7", "0.10", "0.12"} {
		if strings.Contains(row, secret) {
			t.Errorf("the row holds %q in clear text: %s", secret, row)
		}
	}
}

// The chains are per marketplace so that two marketplaces never wait for each
// other, and each one's log is its own (docs/design.md, section 2.6).
func TestEachMarketplaceHasItsOwnChain(t *testing.T) {
	site := assemble(t)
	dbtest.AsApplication(t, site.pool)

	mine := site.marketplace(t, "audit-mine")
	theirs := site.marketplace(t, "audit-theirs")

	site.append(t, mine, changed(person(t), ""))
	site.append(t, mine, changed(person(t), ""))
	site.append(t, theirs, changed(person(t), ""))

	if got := site.length(t, mine); got != 2 {
		t.Errorf("one chain holds %d records, want 2", got)
	}
	if got := site.length(t, theirs); got != 1 {
		t.Errorf("the other holds %d records, want 1", got)
	}

	// And a marketplace's own request sees its own records and no others.
	var visible int
	if err := dbtest.Serving(t, site.pool, theirs, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `SELECT count(*) FROM audit_log`).Scan(&visible)
	}); err != nil {
		t.Fatalf("cannot count the records: %v", err)
	}
	if visible != 1 {
		t.Errorf("a marketplace sees %d records, want its own 1", visible)
	}
}

func (site platform) length(t *testing.T, marketplace string) int64 {
	t.Helper()

	var length int64
	if err := site.pool.InTx(t.Context(), func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(),
			`SELECT length FROM audit_chain WHERE marketplace_id = $1`,
			marketplace).Scan(&length)
	}); err != nil {
		t.Fatalf("cannot read the chain: %v", err)
	}
	return length
}
