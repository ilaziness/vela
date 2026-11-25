package database

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(sql.ErrNoRows) {
		t.Fatal("sql.ErrNoRows should be not found")
	}
	if !IsNotFound(fmt.Errorf("get bookmark failed: %w", sql.ErrNoRows)) {
		t.Fatal("wrapped sql.ErrNoRows should be not found")
	}
	if IsNotFound(errors.New("boom")) {
		t.Fatal("plain error should not be not found")
	}
	if IsNotFound(nil) {
		t.Fatal("nil should not be not found")
	}
}

func TestIsStorageError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("boom"), false},
		{"errNoRows", sql.ErrNoRows, true},
		{"wrapped errNoRows", fmt.Errorf("query bookmarks failed: %w", sql.ErrNoRows), true},
		{"connDone", sql.ErrConnDone, true},
		{"txDone", sql.ErrTxDone, true},
		{"badConn", driver.ErrBadConn, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStorageError(tc.err); got != tc.want {
				t.Fatalf("IsStorageError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsStorageErrorSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Skipf("sqlite driver unavailable: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	// Duplicate primary key yields a *sqlite.Error from the driver.
	if _, err := db.Exec(`INSERT INTO t (id) VALUES (1)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO t (id) VALUES (1)`)
	if err == nil {
		t.Fatal("expected constraint error")
	}
	wrapped := fmt.Errorf("insert bookmark failed: %w", err)
	if !IsStorageError(wrapped) {
		t.Fatalf("wrapped sqlite error not detected: %v", wrapped)
	}
}
