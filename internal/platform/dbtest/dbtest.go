//go:build integration

// Package dbtest gives the integration tests a real PostgreSQL to run against.
//
// The tests that use it are behind the `integration` build tag, so `go test`
// without it stays fast and needs nothing installed (docs/roadmap.md, F4).
//
// Where the database comes from depends on where the tests run. CI starts a
// service container and hands its address over in TEST_DATABASE_URL. A session
// has PostgreSQL installed but nothing running, so this package starts a
// cluster of its own and stops it afterwards. Testcontainers are not an option
// in either place: the Docker daemon does not run in a session
// (docs/roadmap.md, appendix).
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// binDir holds the PostgreSQL programs. The version is pinned to the one the
// design names and CI runs, so a mismatch is a missing directory with a clear
// message rather than a test failing on a syntax the older server lacks.
const binDir = "/usr/lib/postgresql/16/bin"

// owner is the account initdb runs as. It refuses to run as root, and it also
// refuses a data directory root owns, which is why the directory is created
// outside the session's scratchpad and handed over.
const owner = "postgres"

// timeout bounds the calls this package makes on its own account, so that a
// server that accepts a connection and then says nothing fails the run with a
// message instead of holding it until CI gives up.
const timeout = 30 * time.Second

var url string

// Run gives the tests a database of their own, runs them, and takes it away.
//
// Integration tests call it from TestMain. It returns the exit code rather than
// calling os.Exit, so the deferred cleanup actually runs.
func Run(m *testing.M) int {
	server := os.Getenv("TEST_DATABASE_URL")
	if server == "" {
		started, stop, err := start()
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"dbtest: cannot start PostgreSQL: %v\n"+
					"Set TEST_DATABASE_URL to use a database that is already running.\n", err)
			return 1
		}
		defer stop()
		server = started
	}

	own, drop, err := ownDatabase(server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbtest: %v\n", err)
		return 1
	}
	defer drop()

	url = own
	return m.Run()
}

// ownDatabase creates a database for this test binary and returns the function
// that drops it again.
//
// Every package that touches the database applies the migrations, and
// `go test ./...` runs those packages at the same time against the one address
// CI hands over. Sharing a database means two processes both finding a
// migration unapplied and both applying it, and the second one failing on a
// CREATE that has just happened. A database per package removes that, and the
// rest of the class with it: no schema, fixture or grant of one package is
// another package's surprise.
func ownDatabase(server string) (string, func(), error) {
	name, err := databaseName()
	if err != nil {
		return "", nil, err
	}
	quoted := pgx.Identifier{name}.Sanitize() //nolint:misspell // pgx's own method name.

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := onServer(ctx, server, "CREATE DATABASE "+quoted); err != nil {
		return "", nil, fmt.Errorf("cannot create the database %s: %w", name, err)
	}

	own, err := addressOf(server, name)
	if err != nil {
		return "", nil, err
	}

	drop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		// FORCE, because a pool a test forgot to close would otherwise hold
		// the database open and leave it behind on a shared server.
		if err := onServer(ctx, server, "DROP DATABASE IF EXISTS "+quoted+" WITH (FORCE)"); err != nil {
			fmt.Fprintf(os.Stderr, "dbtest: cannot drop the database %s: %v\n", name, err)
		}
	}
	return own, drop, nil
}

// databaseName names the database after the test binary, so that a database
// left behind by a killed run says which package left it, with enough of a
// random tail that two runs of the same package never collide.
func databaseName() (string, error) {
	var tail [6]byte
	if _, err := rand.Read(tail[:]); err != nil {
		return "", fmt.Errorf("cannot name a database: %w", err)
	}

	binary := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, strings.ToLower(filepath.Base(os.Args[0])))

	// PostgreSQL truncates an identifier at 63 bytes, and a truncated name
	// could collide with another package's.
	if len(binary) > 32 {
		binary = binary[:32]
	}
	return "dbtest_" + binary + "_" + hex.EncodeToString(tail[:]), nil
}

// onServer runs one statement on the address the environment gave, which is
// the only database that exists before this package makes one.
func onServer(ctx context.Context, server, statement string) error {
	conn, err := pgx.Connect(ctx, server)
	if err != nil {
		return fmt.Errorf("cannot reach the server: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	_, err = conn.Exec(ctx, statement)
	return err
}

// addressOf is the server's address pointed at another database on it.
func addressOf(server, name string) (string, error) {
	parsed, err := neturl.Parse(server)
	if err != nil {
		return "", fmt.Errorf("the database address is not a URL: %w", err)
	}
	parsed.Path = "/" + name
	return parsed.String(), nil
}

// URL is the connection string the tests connect with.
func URL(tb testing.TB) string {
	tb.Helper()
	if url == "" {
		tb.Fatal("dbtest: no database; the test package must call dbtest.Run from TestMain")
	}
	return url
}

// start brings up a cluster and returns its address and the function that
// stops it again.
func start() (string, func(), error) {
	if _, err := os.Stat(binDir); err != nil {
		return "", nil, fmt.Errorf("%s is not installed: %w", binDir, err)
	}

	// Not the scratchpad: the postgres account has to be able to reach every
	// directory on the path, and a session's scratchpad is not guaranteed to
	// be readable by another user.
	dataDir, err := os.MkdirTemp("/var/tmp", "marketplace-dbtest-")
	if err != nil {
		return "", nil, fmt.Errorf("cannot make a data directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dataDir) }

	if err := run("chown", "-R", owner+":"+owner, dataDir); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(dataDir, 0o750); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("cannot set permissions on the data directory: %w", err)
	}

	// Trust authentication on a loopback socket that exists for the length of
	// one test run. There is no password to configure and none to leak.
	if err := asOwner(binDir+"/initdb", "-D", dataDir, "-U", owner, "--auth=trust", "--no-sync"); err != nil {
		cleanup()
		return "", nil, err
	}

	port, err := freePort()
	if err != nil {
		cleanup()
		return "", nil, err
	}

	options := fmt.Sprintf("-p %d -h 127.0.0.1 -k %s -F", port, dataDir)
	logFile := filepath.Join(dataDir, "server.log")
	if err := asOwner(binDir+"/pg_ctl", "-D", dataDir, "-o", options, "-l", logFile, "-w", "start"); err != nil {
		cleanup()
		return "", nil, err
	}

	stop := func() {
		_ = asOwner(binDir+"/pg_ctl", "-D", dataDir, "-m", "immediate", "-w", "stop")
		cleanup()
	}

	address := fmt.Sprintf("postgres://%s@127.0.0.1:%d/postgres?sslmode=disable", owner, port)
	if err := waitFor(address); err != nil {
		stop()
		return "", nil, err
	}

	return address, stop, nil
}

// asOwner runs a PostgreSQL program as the account that owns the data
// directory, because initdb and pg_ctl refuse to run as root.
func asOwner(program string, args ...string) error {
	quoted := program
	for _, arg := range args {
		quoted += " '" + arg + "'"
	}
	return run("su", owner, "-c", quoted)
}

func run(program string, args ...string) error {
	command := exec.Command(program, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", program, err, output)
	}
	return nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("cannot find a free port: %w", err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// waitFor gives the server a moment to accept connections. pg_ctl -w already
// waits, so this is the second line of defence rather than the first.
func waitFor(address string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := run(binDir+"/pg_isready", "-d", address, "-q"); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("the server did not accept connections in time")
}
