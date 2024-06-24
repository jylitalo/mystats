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

type Record struct {
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
}

type Conditions struct {
	Types    []string
	Workouts []string
	Years    []int
	Month    int
	Day      int
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
	_, err := sq.db.Exec(`create table mystats (
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
		MovingTime  integer
	)`)
	return err
}

func (sq *Sqlite3) Insert(records []Record) error {
	if sq.db == nil {
		return errors.New("database is nil")
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`insert into mystats(Year,Month,Day,Week,StravaID,Name,Type,SportType,WorkoutType,Distance,Elevation,MovingTime) values (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("insert caused %w", err)
	}
	defer stmt.Close()
	for _, r := range records {
		_, err = stmt.Exec(
			r.Year, r.Month, r.Day, r.Week, r.StravaID,
			r.Name, r.Type, r.SportType, r.WorkoutType,
			r.Distance, r.Elevation, r.MovingTime,
		)
		if err != nil {
			return fmt.Errorf("statement execution caused: %w", err)
		}
	}
	return tx.Commit()
}

func sqlQuery(fields []string, cond Conditions, order *Order) string {
	where := []string{}
	if cond.Workouts != nil {
		where = append(where, "(workouttype='"+strings.Join(cond.Workouts, "' or workout_type='")+"')")
	}
	if cond.Types != nil {
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
	return fmt.Sprintf("select %s from mystats%s%s", strings.Join(fields, ","), condition, sorting)
}

func (sq *Sqlite3) Query(fields []string, cond Conditions, order *Order) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query := sqlQuery(fields, cond, order)
	rows, err := sq.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	return rows, err
}

// QueryYears creates list of distinct years from which have records
func (sq *Sqlite3) QueryYears(cond Conditions) ([]int, error) {
	years := []int{}
	rows, err := sq.Query(
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
