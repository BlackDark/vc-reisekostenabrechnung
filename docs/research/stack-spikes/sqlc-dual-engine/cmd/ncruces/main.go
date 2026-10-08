package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/ncruces/go-sqlite3/driver"

	"example.com/dbt/internal/store/sqlitedb"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	db, err := sql.Open("sqlite3", "file:/tmp/dbt/n.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
	must(err)
	goose.SetBaseFS(migrations)
	must(goose.SetDialect("sqlite3"))
	must(goose.Up(db, "migrations"))
	run(db)
}
func run(db *sql.DB) {
	q := sqlitedb.New(db)
	ctx := context.Background()
	t := time.Now()
	tx, _ := db.Begin()
	qt := q.WithTx(tx)
	for i := 0; i < 20000; i++ {
		_, err := qt.CreateTrip(ctx, sqlitedb.CreateTripParams{ID: fmt.Sprint(time.Now().UnixNano(), i), UserID: "u1", Title: "x", StartDate: "2026-10-01", EndDate: "2026-10-02", KmTotal: 10, AmountCents: 1234})
		must(err)
	}
	must(tx.Commit())
	ins := time.Since(t)
	t = time.Now()
	rows, err := q.ListTripsInPeriod(ctx, sqlitedb.ListTripsInPeriodParams{UserID: "u1", FromDate: "2026-01-01", ToDate: "2026-12-31", Lim: 1000})
	must(err)
	s, _ := q.SumAmount(ctx, "u1")
	var mode string
	db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	fmt.Println("insert20k", ins, "list", len(rows), time.Since(t), "sum", s, "mode", mode)
}
func must(err error) { if err != nil { panic(err) } }
