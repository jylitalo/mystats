package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	garmin "github.com/jylitalo/go-garmin"
	"github.com/jylitalo/mystats/pkg/telemetry"
	_ "github.com/mattn/go-sqlite3"
)

// Strava
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

// Garmin
type DailyStepsRecord struct {
	Year       int
	Month      int
	Day        int
	Week       int
	TotalSteps int
}

type HeartRateRecord struct {
	WellnessMaxAvgHR int
	WellnessMinAvgHR int
	RestingHR        int
}

type OrderConfig struct {
	GroupBy []string
	OrderBy []string
	Limit   int
}

type Sqlite3 struct {
	db *sql.DB
}

type QueryConfig struct {
	Tables   []string
	Name     string
	StravaID int64
	Day      int
	Month    int
	Years    []int
	Sport    []string
	Workout  []string
	Order    *OrderConfig
}

type QueryOption func(c *QueryConfig)

func WithTable(table string) QueryOption {
	return func(c *QueryConfig) {
		if slices.Contains(c.Tables, table) {
			slog.Error("WithTable already contains table", "c.Tables", c.Tables, "table", table)
		}
		c.Tables = append(c.Tables, table)
	}
}

func WithDayOfYear(day, month int) QueryOption {
	return func(c *QueryConfig) {
		c.Day = day
		c.Month = month
	}
}

func WithYear(year int) QueryOption {
	return func(c *QueryConfig) {
		c.Years = append(c.Years, year)
	}
}

func WithOrder(order OrderConfig) QueryOption {
	return func(c *QueryConfig) {
		c.Order = &order
	}
}

func WithStravaID(number int64) QueryOption {
	return func(c *QueryConfig) {
		c.StravaID = number
	}
}

func WithName(name string) QueryOption {
	return func(c *QueryConfig) {
		c.Name = name
	}
}

func WithSport(name string) QueryOption {
	return func(c *QueryConfig) {
		c.Sport = append(c.Sport, name)
	}
}

func WithWorkout(name string) QueryOption {
	return func(c *QueryConfig) {
		c.Workout = append(c.Workout, name)
	}
}

const dbName string = "mystats.sql"
const BestEffortTable string = "BestEffort"
const DailyStepsTable string = "DailySteps"
const HeartRateTable string = "HeartRate"
const SplitTable string = "Split"
const SummaryTable string = "Summary"

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
	ymdw := "Year integer, Month integer, Day integer, Week integer,"
	stravaId := "StravaID integer,"
	emd := "ElapsedTime integer, MovingTime integer, Distance integer,"
	_, errSummary := sq.db.Exec(`create table ` + SummaryTable + ` ( ` + ymdw + stravaId + emd + `
		Name        text,
		Type        text,
		SportType   text,
		WorkoutType text,
		Elevation   real
	)`)
	_, errBE := sq.db.Exec(`create table ` + BestEffortTable + ` ( ` + stravaId + emd + `
		Name        text
	)`)
	_, errSplit := sq.db.Exec(`create table ` + SplitTable + ` ( ` + stravaId + emd + `
		Split         integer,
		ElevationDiff real
	)`)
	_, errSteps := sq.db.Exec(`create table ` + DailyStepsTable + ` ( ` + ymdw + `
		TotalSteps  integer,
		StepGoal    integer
	)`)
	_, errHeartRate := sq.db.Exec(`create table ` + HeartRateTable + ` ( ` + ymdw + `
		WellnessMinAvgHR integer,
		WellnessMaxAvgHR integer,
		RestingHR integer
	)`)
	return errors.Join(errSummary, errBE, errHeartRate, errSplit, errSteps)
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
	fields := []string{
		"Year", "Month", "Day", "Week", "StravaID", "Name", "Type", "SportType", "WorkoutType",
		"Distance", "Elevation", "ElapsedTime", "MovingTime",
	}
	q := strings.Repeat("?,", len(fields)-1) + "?"
	stmt, err := tx.Prepare("insert into " + SummaryTable + "(" + strings.Join(fields, ",") + ") values (" + q + ")")
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("InsertSummary caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		_, err = stmt.Exec(
			r.Year, r.Month, r.Day, r.Week, r.StravaID,
			r.Name, r.Type, r.SportType, r.WorkoutType,
			r.Distance, r.Elevation, r.ElapsedTime, r.MovingTime,
		)
		if err != nil {
			return fmt.Errorf("InsertSummary statement execution caused: %w", err)
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
	fields := []string{"StravaID", "Name", "ElapsedTime", "MovingTime", "Distance"}
	q := strings.Repeat("?,", len(fields)-1) + "?"
	stmt, err := tx.Prepare("insert into " + BestEffortTable + "(" + strings.Join(fields, ",") + ") values (" + q + ")")
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("InsertBestEffort caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err = stmt.Exec(r.StravaID, r.Name, r.ElapsedTime, r.MovingTime, r.Distance); err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertBestEffort statement execution caused: %w", err))
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
	fields := []string{"StravaID", "Split", "ElapsedTime", "MovingTime", "Distance", "ElevationDiff"}
	q := strings.Repeat("?,", len(fields)-1) + "?"
	stmt, err := tx.Prepare("insert into " + SplitTable + "(" + strings.Join(fields, ",") + ") values (" + q + ")")
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("InsertSplit caused %w", err))
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err = stmt.Exec(r.StravaID, r.Split, r.ElapsedTime, r.MovingTime, r.Distance, r.ElevationDiff); err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertSplit statement execution caused: %w", err))
		}
	}
	return telemetry.Error(span, tx.Commit())
}

func (sq *Sqlite3) InsertDailySteps(ctx context.Context, records map[string]garmin.DailyStepsStat) error {
	_, span := telemetry.NewSpan(ctx, "InsertDailySteps")
	defer span.End()
	// slog.Info("storage.InsertDailySteps", "records", records)
	if sq.db == nil {
		return telemetry.Error(span, errors.New("database is nil"))
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return telemetry.Error(span, err)
	}
	fields := []string{"Year", "Month", "Day", "Week", "TotalSteps", "StepGoal"}
	q := strings.Repeat("?,", len(fields)-1) + "?"
	stmt, err := tx.Prepare("insert into " + DailyStepsTable + "(" + strings.Join(fields, ",") + ") values (" + q + ")")
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("InsertDailySteps caused %w", err))
	}
	for key, r := range records {
		t, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertDailySteps time parsing (%s) caused: %w", key, err))
		}
		_, week := t.ISOWeek()
		if _, err = stmt.Exec(t.Year(), t.Month(), t.Day(), week, r.TotalSteps, r.StepGoal); err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertDailySteps statement execution caused: %w", err))
		}
	}
	defer stmt.Close()
	return telemetry.Error(span, tx.Commit())
}

func (sq *Sqlite3) InsertHeartRate(ctx context.Context, records map[string]garmin.HeartRateStat) error {
	_, span := telemetry.NewSpan(ctx, "InsertHeartRate")
	defer span.End()
	slog.Info("storage.InsertHeartRate", "records", records)
	if sq.db == nil {
		return telemetry.Error(span, errors.New("database is nil"))
	}
	tx, err := sq.db.Begin()
	if err != nil {
		return telemetry.Error(span, err)
	}
	fields := []string{"Year", "Month", "Day", "Week", "WellnessMinAvgHR", "WellnessMaxAvgHR", "RestingHR"}
	q := strings.Repeat("?,", len(fields)-1) + "?"
	stmt, err := tx.Prepare("insert into " + HeartRateTable + "(" + strings.Join(fields, ",") + ") values (" + q + ")")
	if err != nil {
		return telemetry.Error(span, fmt.Errorf("InsertHeartRate caused %w", err))
	}
	for key, r := range records {
		slog.Info("InsertHeartRate", "key", key, "r", r)
		t, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertHeartRate time parsing (%s) caused: %w", key, err))
		}
		_, week := t.ISOWeek()
		if _, err = stmt.Exec(t.Year(), t.Month(), t.Day(), week, r.WellnessMinAvgHR, r.WellnessMaxAvgHR, r.RestingHR); err != nil {
			return telemetry.Error(span, fmt.Errorf("InsertHeartRate statement execution caused: %w", err))
		}
	}
	defer stmt.Close()
	return telemetry.Error(span, tx.Commit())
}

func sqlQuery(fields []string, opts ...QueryOption) (string, []interface{}) {
	cfg := &QueryConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	where := []string{}
	args := []string{}
	if len(cfg.Tables) == 0 {
		panic("no tables specified in sqlQuery")
	}
	if len(cfg.Tables) > 1 {
		for _, table := range cfg.Tables[1:] {
			where = append(where, fmt.Sprintf("%s.StravaID=%s.StravaID", cfg.Tables[0], table))
		}
	}
	if len(cfg.Workout) > 0 {
		where = append(where, "(Workouttype="+strings.Repeat("? or Workouttype=", len(cfg.Workout)-1)+"?)")
		args = append(args, cfg.Workout...)
	}
	if len(cfg.Sport) > 0 {
		where = append(where, "(Type="+strings.Repeat("? or Type=", len(cfg.Sport)-1)+"?)")
		args = append(args, cfg.Sport...)
	}
	if cfg.Month > 0 && cfg.Day > 0 {
		where = append(where, "(Month < ? or (Month=? and Day<=?))")
		month := strconv.Itoa(cfg.Month)
		args = append(args, month, month, strconv.Itoa(cfg.Day))
	}
	if len(cfg.Years) > 0 {
		where = append(where, "(Year="+strings.Repeat("? or Year=", len(cfg.Years)-1)+"?)")
		for _, y := range cfg.Years {
			args = append(args, strconv.Itoa(y))
		}
	}
	if cfg.Name != "" {
		if slices.Contains(cfg.Tables, BestEffortTable) {
			where = append(where, BestEffortTable+".Name=?")
			args = append(args, cfg.Name)
		} else {
			where = append(where, SummaryTable+".Name LIKE ?")
			args = append(args, cfg.Name)
		}
	}
	if cfg.StravaID > 0 {
		for _, t := range cfg.Tables {
			where = append(where, t+".stravaid=?")
			args = append(args, strconv.FormatInt(cfg.StravaID, 10))
		}
	}
	condition := ""
	if len(where) > 0 {
		condition = " where " + strings.Join(where, " and ")
	}
	sorting := ""
	if cfg.Order != nil {
		if cfg.Order.GroupBy != nil {
			sorting += " group by " + strings.Join(cfg.Order.GroupBy, ",")
		}
		if cfg.Order.OrderBy != nil {
			sorting += " order by " + strings.Join(cfg.Order.OrderBy, ",")
		}
		if cfg.Order.Limit > 0 {
			sorting += " limit " + strconv.FormatInt(int64(cfg.Order.Limit), 10)
		}
	}
	ifArgs := make([]interface{}, len(args))
	for i, v := range args {
		ifArgs[i] = v
	}
	return fmt.Sprintf(
		"select %s from %s%s%s", strings.Join(fields, ","), strings.Join(cfg.Tables, ","),
		condition, sorting,
	), ifArgs
}

func (sq *Sqlite3) QueryBestEffortDistances() ([]string, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery(
		[]string{"distinct(" + BestEffortTable + ".Name)"},
		WithTable(BestEffortTable),
		WithOrder(OrderConfig{OrderBy: []string{"distance desc"}}),
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

// QueryTypes creates list of distinct years from which have records
func (sq *Sqlite3) QuerySports() ([]string, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery(
		[]string{"distinct(" + SummaryTable + ".Type)"},
		WithTable(SummaryTable),
		WithOrder(OrderConfig{GroupBy: []string{"type"}, OrderBy: []string{"type"}}),
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	sports := []string{}
	if err != nil {
		return sports, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return sports, err
		}
		sports = append(sports, value)
	}
	return sports, nil
}

// QueryTypes creates list of distinct years from which have records
func (sq *Sqlite3) QueryWorkouts() ([]string, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	query, values := sqlQuery(
		[]string{"distinct(" + SummaryTable + ".WorkoutType)"},
		WithTable(SummaryTable),
		WithOrder(OrderConfig{GroupBy: []string{SummaryTable + ".WorkoutType"}, OrderBy: []string{SummaryTable + ".WorkoutType"}}),
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	workouts := []string{}
	if err != nil {
		return workouts, fmt.Errorf("select caused: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return workouts, err
		}
		workouts = append(workouts, value)
	}
	return workouts, nil
}

// QueryYears creates list of distinct years from which have records
func (sq *Sqlite3) QueryYears(opts ...QueryOption) ([]int, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	defaultOpts := []QueryOption{
		WithOrder(OrderConfig{GroupBy: []string{"Year"}, OrderBy: []string{"Year desc"}}),
	}
	if len(opts) == 0 {
		defaultOpts = append(defaultOpts, WithTable(SummaryTable))
	}
	opts = append(defaultOpts, opts...)
	query, values := sqlQuery(
		[]string{"distinct(Year)"}, opts...,
	)
	// slog.Info("storage.Query", "query", query)
	rows, err := sq.db.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", query, err)
	}
	defer rows.Close()
	years := []int{}
	for rows.Next() {
		var year int
		if err = rows.Scan(&year); err != nil {
			return years, err
		}
		years = append(years, year)
	}
	return years, nil
}

func (sq *Sqlite3) Query(fields []string, opts ...QueryOption) (*sql.Rows, error) {
	if sq.db == nil {
		return nil, errors.New("database is nil")
	}
	defaultOpts := []QueryOption{
		WithOrder(OrderConfig{GroupBy: []string{"Year"}, OrderBy: []string{"Year desc"}}),
	}
	if len(opts) == 0 {
		defaultOpts = append(defaultOpts, WithTable(SummaryTable))
	}
	query, values := sqlQuery(fields, opts...)
	return sq.db.Query(query, values...)
}

func (sq *Sqlite3) Close() error {
	if sq.db != nil {
		return sq.db.Close()
	}
	return nil
}
