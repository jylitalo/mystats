package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jylitalo/mystats/pkg/telemetry"
	_ "github.com/mattn/go-sqlite3"
)

type SummaryRecord struct {
	Year        int
	Month       int
	Day         int
	Week        int
	StravaID    int64
	Name        string
	Type        string
	SportType   string
	WorkoutType string
	Distance    float64
	Elevation   float64
	MovingTime  int
	ElapsedTime int
}

type BestEffortRecord struct {
	StravaID    int64
	Name        string
	ElapsedTime int
	MovingTime  int
	Distance    int
}

type SplitRecord struct {
	StravaID      int64
	Split         int
	ElapsedTime   int
	MovingTime    int
	ElevationDiff float64
	Distance      float64
}

type SummaryConditions struct {
	Types        []string
	WorkoutTypes []string
	Years        []int
	Month        int
	Day          int
	Name         string
	StravaID     int64
}

type conditions struct {
	Types        []string
	WorkoutTypes []string
	Years        []int
	Month        int
	Day          int
	BEName       string
	Name         string
	StravaID     int64
}

type Order struct {
	GroupBy []string
	OrderBy []string
	Limit   int
}

type Sqlite3 struct {
	db *sql.DB
}

const dbName string = "mystats.sql"

func (sq *Sqlite3) Remove() error {
	if _, err := os.Stat(dbName); err != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return os.Remove(dbName)
}

// LastModified returns error or it will tell when database was last modified
func (sq *Sqlite3) LastModified() (time.Time, error) {
	fi, err := os.Stat(dbName)
	if err != nil {
		epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
		return epoch, err
	}
	return fi.ModTime().UTC(), nil
}

func (sq *Sqlite3) Open() error {
	var err error

	if sq.db == nil {
		sq.db, err = sql.Open("sqlite3", dbName)
	}
	return err
}

func (sq *Sqlite3) Create() error {
	if sq.db == nil {
		return errors.New("database is nil")
	}
	_, errSummary := sq.db.Exec(`create table Summary (
		Year        integer,
		Month       integer,
		Day         integer,
		Week        integer,
		StravaID    integer,
		Name        text,
		Type        text,
		SportType   text,
		WorkoutType text,
		Distance    real,
		Elevation   real,
		ElapsedTime integer,
		MovingTime  integer
	)`)
	_, errBE := sq.db.Exec(`create table BestEffort (
		StravaID    integer,
		Name        text,
		ElapsedTime integer,
		MovingTime  integer,
		Distance    integer
	)`)
	_, errSplit := sq.db.Exec(`create table Split (
		StravaID      integer,
		Split         integer,
		ElapsedTime   integer,
		MovingTime    integer,
		Distance      real,
		ElevationDiff real
	)`)
	return errors.Join(errSummary, errBE, errSplit)
}

func (sq *Sqlite3) InsertSummary(ctx context.Context, records []SummaryRecord) error {
	_, span := telemetry.NewSpan(ctx, "InsertSummary")
	defer span.End()
	if sq.db == nil {
		return telemetry.Error(span, errors.New("database is nil"))
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return telemetry.Error(span, err)
	}
	stmt, err := tx.Prepare(`insert into summary(Year,Month,Day,Week,StravaID,Name,Type,SportType,WorkoutType,Distance,Elevation,ElapsedTime,MovingTime) values (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("insert caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		_, err = stmt.Exec(
			r.Year, r.Month, r.Day, r.Week, r.StravaID,
			r.Name, r.Type, r.SportType, r.WorkoutType,
			r.Distance, r.Elevation, r.ElapsedTime, r.MovingTime,
		)
		if err != nil {
			return fmt.Errorf("statement execution caused: %w", err)
		}
	}
	return telemetry.Error(span, tx.Commit())
}

func (sq *Sqlite3) InsertBestEffort(ctx context.Context, records []BestEffortRecord) error {
	_, span := telemetry.NewSpan(ctx, "InsertBestEffort")
	defer span.End()
	if sq.db == nil {
		return telemetry.Error(span, errors.New("database is nil"))
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return telemetry.Error(span, err)
	}
	stmt, err := tx.Prepare(`insert into BestEffort(StravaID,Name,ElapsedTime,MovingTime,Distance) values (?,?,?,?,?)`)
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("insert caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err = stmt.Exec(r.StravaID, r.Name, r.ElapsedTime, r.MovingTime, r.Distance); err != nil {
			return telemetry.Error(span, fmt.Errorf("statement execution caused: %w", err))
		}
	}
	return telemetry.Error(span, tx.Commit())
}

func (sq *Sqlite3) InsertSplit(ctx context.Context, records []SplitRecord) error {
	_, span := telemetry.NewSpan(ctx, "InsertSplit")
	defer span.End()
	if sq.db == nil {
		return telemetry.Error(span, errors.New("database is nil"))
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return telemetry.Error(span, err)
	}
	stmt, err := tx.Prepare(`insert into Split(StravaID,Split,ElapsedTime,MovingTime,Distance,ElevationDiff) values (?,?,?,?,?,?)`)
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("insert caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err = stmt.Exec(r.StravaID, r.Split, r.ElapsedTime, r.MovingTime, r.Distance, r.ElevationDiff); err != nil {
			return telemetry.Error(span, fmt.Errorf("statement execution caused: %w", err))
		}
	}
	return telemetry.Error(span, tx.Commit())
}

func sqlQuery(tables []string, fields []string, cond conditions, order *Order) (string, []interface{}) {
	where := []string{}
	args := []string{}
	if len(tables) > 0 {
		for _, table := range tables[1:] {
			where = append(where, fmt.Sprintf("%s.StravaID=%s.StravaID", tables[0], table))
		}
	}
	if len(cond.WorkoutTypes) > 0 {
		where = append(where, "(workouttype="+strings.Repeat("? or workouttype=", len(cond.WorkoutTypes)-1)+"?)")
		args = append(args, cond.WorkoutTypes...)
	}
	if len(cond.Types) > 0 {
		where = append(where, "(type="+strings.Repeat("? or type=", len(cond.Types)-1)+"?)")
		args = append(args, cond.Types...)
	}
	if cond.Month > 0 && cond.Day > 0 {
		where = append(where, "(month < ? or (month=? and day<=?))")
		month := strconv.Itoa(cond.Month)
		args = append(args, month, month, strconv.Itoa(cond.Day))
	}
	if len(cond.Years) > 0 {
		where = append(where, "(year="+strings.Repeat("? or year=", len(cond.Years)-1)+"?)")
		for _, y := range cond.Years {
			args = append(args, strconv.Itoa(y))
		}
	}
	if cond.BEName != "" {
		where = append(where, "besteffort.name=?")
		args = append(args, cond.BEName)
	}
	if cond.StravaID > 0 {
		for _, t := range tables {
			where = append(where, t+".stravaid=?")
			args = append(args, strconv.FormatInt(cond.StravaID, 10))
		}
	}
	if cond.Name != "" {
		where = append(where, "summary.name LIKE ?")
		args = append(args, cond.Name)
	}
	condition := ""
	if len(where) > 0 {
		condition = " where " + strings.Join(where, " and ")
	}
	sorting := ""
	if order != nil {
		if order.GroupBy != nil {
			sorting += " group by " + strings.Join(order.GroupBy, ",")
		}
		if order.OrderBy != nil {
			sorting += " order by " + strings.Join(order.OrderBy, ",")
		}
		if order.Limit > 0 {
			sorting += " limit " + strconv.FormatInt(int64(order.Limit), 10)
		}
	}
	ifArgs := make([]interface{}, len(args))
	for i, v := range args {
		ifArgs[i] = v
	}
	return fmt.Sprintf(
		"select %s from %s%s%s", strings.Join(fields, ","), strings.Join(tables, ","),
		condition, sorting,
	), ifArgs
}

func (sq *Sqlite3) QueryBestEffort(fields []string, name string, order *Order) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery([]string{"besteffort", "summary"}, fields, conditions{BEName: name}, order)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	return rows, err
}

func (sq *Sqlite3) QueryBestEffortDistances() ([]string, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery(
		[]string{"besteffort"}, []string{"distinct(name)"}, conditions{},
		&Order{OrderBy: []string{"distance desc"}},
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	defer rows.Close()
	benames := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return benames, err
		}
		benames = append(benames, value)
	}
	// slog.Info("QueryBestEffortDistances", "benames", benames)
	return benames, nil
}

func (sq *Sqlite3) QuerySplit(fields []string, id int64) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery(
		[]string{"split"}, fields, conditions{StravaID: id}, &Order{OrderBy: []string{"split"}},
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	return rows, err
}

func (sq *Sqlite3) QuerySummary(fields []string, sumCond SummaryConditions, order *Order) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	genCond := conditions{
		Types: sumCond.Types, WorkoutTypes: sumCond.WorkoutTypes,
		Years: sumCond.Years, Month: sumCond.Month, Day: sumCond.Day,
		Name: sumCond.Name, StravaID: sumCond.StravaID,
	}
	query, values := sqlQuery([]string{"summary"}, fields, genCond, order)
	// slog.Info("storage.QuerySummary", "query", query, "cond", sumCond, "values", values)
	rows, err := sq.db.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	return rows, err
}

// QueryTypes creates list of distinct years from which have records
func (sq *Sqlite3) QueryTypes(cond SummaryConditions) ([]string, error) {
	types := []string{}
	rows, err := sq.QuerySummary(
		[]string{"distinct(type)"}, cond,
		&Order{GroupBy: []string{"type"}, OrderBy: []string{"type"}},
	)
	if err != nil {
		return types, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return types, err
		}
		types = append(types, value)
	}
	return types, nil
}

// QueryTypes creates list of distinct years from which have records
func (sq *Sqlite3) QueryWorkoutTypes(cond SummaryConditions) ([]string, error) {
	types := []string{}
	rows, err := sq.QuerySummary(
		[]string{"distinct(workouttype)"}, cond,
		&Order{GroupBy: []string{"workouttype"}, OrderBy: []string{"workouttype"}},
	)
	if err != nil {
		return types, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return types, err
		}
		types = append(types, value)
	}
	return types, nil
}

// QueryYears creates list of distinct years from which have records
func (sq *Sqlite3) QueryYears(cond SummaryConditions) ([]int, error) {
	years := []int{}
	rows, err := sq.QuerySummary(
		[]string{"distinct(year)"}, cond,
		&Order{GroupBy: []string{"year"}, OrderBy: []string{"year desc"}},
	)
	if err != nil {
		return years, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var year int
		if err = rows.Scan(&year); err != nil {
			return years, err
		}
		years = append(years, year)
	}
	return years, nil
}

func (sq *Sqlite3) Close() error {
	if sq.db != nil {
		return sq.db.Close()
	}
	return nil
}
