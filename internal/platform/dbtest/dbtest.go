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
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// binDir holds the PostgreSQL programs. The version is pinned to the one the
// design names and CI runs, so a mismatch is a missing directory with a clear
// message rather than a test failing on a syntax the older server lacks.
const binDir = "/usr/lib/postgresql/16/bin"

// owner is the account initdb runs as. It refuses to run as root, and it also
// refuses a data directory root owns, which is why the directory is created
// outside the session's scratchpad and handed over.
const owner = "postgres"

var url string

// Run starts a database, runs the tests, and stops it again.
//
// Integration tests call it from TestMain. It returns the exit code rather than
// calling os.Exit, so the deferred cleanup actually runs.
func Run(m *testing.M) int {
	if fromEnvironment := os.Getenv("TEST_DATABASE_URL"); fromEnvironment != "" {
		url = fromEnvironment
		return m.Run()
	}

	dataDir, stop, err := start()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"dbtest: cannot start PostgreSQL: %v\n"+
				"Set TEST_DATABASE_URL to use a database that is already running.\n", err)
		return 1
	}
	defer stop()

	_ = dataDir
	return m.Run()
}

// URL is the connection string the tests connect with.
func URL(tb testing.TB) string {
	tb.Helper()
	if url == "" {
		tb.Fatal("dbtest: no database; the test package must call dbtest.Run from TestMain")
	}
	return url
}

// start brings up a cluster and returns the function that stops it.
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

	url = fmt.Sprintf("postgres://%s@127.0.0.1:%d/postgres?sslmode=disable", owner, port)
	if err := waitFor(url); err != nil {
		stop()
		return "", nil, err
	}

	return dataDir, stop, nil
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
func waitFor(url string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := run(binDir+"/pg_isready", "-d", url, "-q"); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("the server did not accept connections in time")
}
