package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

type SummaryConditions struct {
	Types        []string
	WorkoutTypes []string
	Years        []int
	Month        int
	Day          int
}

type conditions struct {
	Types        []string
	WorkoutTypes []string
	Years        []int
	Month        int
	Day          int
	BEName       string
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
	_, errS := sq.db.Exec(`create table Summary (
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
	return errors.Join(errS, errBE)
}

func (sq *Sqlite3) InsertSummary(records []SummaryRecord) error {
	if sq.db == nil {
		return errors.New("database is nil")
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`insert into summary(Year,Month,Day,Week,StravaID,Name,Type,SportType,WorkoutType,Distance,Elevation,ElapsedTime,MovingTime) values (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("insert caused %w", err)
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
	return tx.Commit()
}

func (sq *Sqlite3) InsertBestEffort(records []BestEffortRecord) error {
	if sq.db == nil {
		return errors.New("database is nil")
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`insert into BestEffort(StravaID,Name,ElapsedTime,MovingTime,Distance) values (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("insert caused %w", err)
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err = stmt.Exec(r.StravaID, r.Name, r.ElapsedTime, r.MovingTime, r.Distance); err != nil {
			return fmt.Errorf("statement execution caused: %w", err)
		}
	}
	return tx.Commit()
}

func sqlQuery(tables []string, fields []string, cond conditions, order *Order) string {
	where := []string{}
	if len(tables) > 0 {
		for _, table := range tables[1:] {
			where = append(where, fmt.Sprintf("%s.StravaID=%s.StravaID", tables[0], table))
		}
	}
	if len(cond.WorkoutTypes) > 0 {
		where = append(where, "(workouttype='"+strings.Join(cond.WorkoutTypes, "' or workouttype='")+"')")
	}
	if len(cond.Types) > 0 {
		where = append(where, "(type='"+strings.Join(cond.Types, "' or type='")+"')")
	}
	if cond.Month > 0 && cond.Day > 0 {
		where = append(where, fmt.Sprintf("(month < %d or (month=%d and day<=%d))", cond.Month, cond.Month, cond.Day))
	}
	if len(cond.Years) > 0 {
		yearStr := []string{}
		for _, y := range cond.Years {
			yearStr = append(yearStr, strconv.Itoa(y))
		}
		where = append(where, "(year="+strings.Join(yearStr, " or year=")+")")
	}
	if cond.BEName != "" {
		where = append(where, "besteffort.name='"+cond.BEName+"'")
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
	return fmt.Sprintf(
		"select %s from %s%s%s", strings.Join(fields, ","), strings.Join(tables, ","),
		condition, sorting,
	)
}

func (sq *Sqlite3) QueryBestEffort(fields []string, name string, order *Order) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query := sqlQuery([]string{"besteffort", "summary"}, fields, conditions{BEName: name}, order)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	return rows, err
}

func (sq *Sqlite3) QueryBestEffortDistances() ([]string, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query := sqlQuery(
		[]string{"besteffort"}, []string{"distinct(name)"}, conditions{},
		&Order{OrderBy: []string{"distance desc"}},
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query)
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

func (sq *Sqlite3) QuerySummary(fields []string, cond SummaryConditions, order *Order) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query := sqlQuery(
		[]string{"summary"}, fields,
		conditions{
			Types: cond.Types, WorkoutTypes: cond.WorkoutTypes,
			Years: cond.Years, Month: cond.Month, Day: cond.Day,
		},
		order,
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query)
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
