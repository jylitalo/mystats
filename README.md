# mystats - Analyze your Strava activities

## Setup

1. `go build -o mystats main.go`
2. Go to https://www.strava.com/settings/api to setup API access for yourself
3. `./mystats configure --client_id ... --client_secret ...
   1. It will instruct you to enter URL to browser
   2. Authorize your app in browser
   3. Browser will redirect you to address that can't be found, but copy paste it to your app and app will write your configuration file into ~/.mystats.yaml
4. `./mystats fetch` will fetch your activities into pages subdirectory in JSON files
5. `./mystats make` will transform JSON files from pages directory into sqlite3

## Commands

- `list` output matching activities
- `stats` aggregate weekly/monthly stats
- `top` list weeks/months with highest numbers
- `plot` cumulative sum of activities in various years

## Examples

### stats

```
 % ./mystats stats --format=table --period=month
+-------+------+------+------+------+------+------+------+------+------+
| MONTH | 2024 | 2023 | 2022 | 2021 | 2020 | 2019 | 2018 | 2017 | 2016 |
+-------+------+------+------+------+------+------+------+------+------+
|     1 |   92 |  119 |  136 |  128 |   71 |   91 |  119 |   59 |      |
|     2 |   21 |  139 |  111 |  105 |   65 |   82 |   55 |   37 |      |
|     3 |  119 |  201 |  157 |  118 |   93 |  118 |   33 |   37 |      |
|     4 |  161 |  161 |  176 |  163 |  167 |  118 |   94 |   53 |      |
|     5 |   65 |  161 |  193 |  125 |    9 |  101 |  151 |   74 |      |
|     6 |      |   53 |   88 |   98 |   17 |   92 |  114 |   73 |    5 |
|     7 |      |  142 |  125 |   75 |   98 |   90 |   63 |   96 |   63 |
|     8 |      |  173 |  180 |  168 |  113 |   78 |   91 |   82 |  108 |
|     9 |      |  143 |  125 |  138 |  162 |   91 |   25 |   99 |   47 |
|    10 |      |  100 |  165 |  149 |  102 |   68 |  127 |   85 |  111 |
|    11 |      |   78 |   50 |   43 |  116 |   71 |   93 |   84 |   30 |
|    12 |      |   22 |   23 |   34 |  123 |   25 |   56 |   22 |    7 |
+-------+------+------+------+------+------+------+------+------+------+
| TOTAL |  458 | 1491 | 1527 | 1345 | 1136 | 1024 | 1021 |  802 |  371 |
+-------+------+------+------+------+------+------+------+------+------+
```

### top

```
% ./mystats top --format=table --limit 5
+---------------+------+------+
| SUM(DISTANCE) | YEAR | WEEK |
+---------------+------+------+
| 86.1km        | 2022 |   21 |
| 66.9km        | 2023 |   40 |
| 58.9km        | 2019 |   28 |
| 56.9km        | 2023 |   31 |
| 56.1km        | 2019 |   21 |
+---------------+------+------+
```
