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

