# ngtop

ngtop is a command-line program to query request counts from nginx's access.log files.

```
$ ngtop url user_agent --since 1d --where url=/blog/% --where status=200 --limit 5
PATH                                             USER_AGENT     #REQS
/blog/deconstructing-the-role-playing-videogame/ Safari         120
/blog/on-ai-assistance/                          Go-http-client 101
/blog/a-note-on-essential-complexity/            Safari         91
/blog/deconstructing-the-role-playing-videogame/ Chrome         84
/blog/from-rss-to-my-kindle/                     Safari         79
```

## Installation

Download the [latest release binary](https://github.com/facundoolano/ngtop/releases/latest) for your platform, for example:

    $ wget https://github.com/facundoolano/ngtop/releases/latest/download/ngtop-linux-arm64  \
        -O ngtop && chmod +x ngtop && mv ngtop /usr/local/bin

Alternatively, install with go:

    $ go install github.com/facundoolano/ngtop@latest

## Usage examples

Count requests from the last hour:

    $ ngtop --since 1h
    $ ngtop -s 1h
    $ ngtop

Count requests from the last second, minute, day, week, or month:

    $ ngtop -s 1s
    $ ngtop -s 1m
    $ ngtop -s 1d
    $ ngtop -s 1w
    $ ngtop -s 1M

Count requests from the day before:

    $ ngtop --since 2d --until 1d
    $ ngtop -s 2d -u 1d

Show the top 5 urls in the last hour:

    $ ngtop url
    $ ngtop path

Show the top 5 urls in the last minute:

    $ ngtop url -s 1m

Show the top 10 urls in the last hour:

    $ ngtop url --limit 10
    $ ngtop url -l 10

Count total requests to a specific url in the last hour:

	$ ngtop --where url=/blog/code-is-run-more-than-read
	$ ngtop --where path=/blog/code-is-run-more-than-read
	$ ngtop -w url=/blog/code-is-run-more-than-read

Count total requests to urls matching a pattern:

	$ ngtop -w url=/blog/%

Count total requests to urls excluding a value or pattern:

	$ ngtop -w url!=/feed.xml
	$ ngtop -w url!=/feed%

Count total requests to one of mutliple urls (one OR another):

	$ ngtop -w url=/blog/code-is-run-more-than-read -w url=/blog/a-note-on-essential-complexity

Count total requests to a specific urls AND referer:

	$ ngtop -w url=/blog/code-is-run-more-than-read -w referer=news.ycombinator.com

Show the top visited urls matching a pattern:

	$ ngtop url -w url=/blog/%

Show the top requesting ips:

    $ ngtop ip

Show the top url visits by ip:

    $ ngtop url -w ip=77.16.76.86

Show the top user agents by url:

    $ ngtop user_agent -w url=/blog/code-is-run-more-than-read
    $ ngtop ua -w url=/blog/code-is-run-more-than-read

Show the top urls by user agent details (parsed with [mileusna/useagent](https://pkg.go.dev/github.com/mileusna/useragent)):

    $ ngtop url -w ua=Firefox
    $ ngtop url -w ua_type=bot
    $ ngtop url -w device=iPhone
    $ ngtop url -w os=Linux

Show the top referers for a url pattern:

    $ ngtop referer -w url=/blog/%

Show the top user agent and referer combination

    $ ngtop ua referer

Show the top user agent and referer combination for a specific url

    $ ngtop ua referer -w url=/blog/code-is-run-more-than-read

Count total 404 status responses:

    $ ngtop -w status=404

Count non successful responses:

    $ ngtop status -w status=4% -status=5%

## How it works

- Whenever the program is run, it looks for the nginx access.logs, parses them and stores the data into an SQLite DB.
  - By default, the logs are looked up at `/var/log/nginx/access.log*`, which can be overridden with the `NGTOP_LOGS_PATH` environment variable.
  - By default, the logs are assumed to have the [nginx combined log format](https://nginx.org/en/docs/http/ngx_http_log_module.html#log_format). The format can be customized with `NGTOP_LOG_FORMAT`.
    - This could likely be made to work with non nginx logs, although that hasn't been tested.
  - Subsequent runs of the program only parse and store the logs up until the time of the previous run.
  - The SQLite DB is stored at `./ngtop.db`, which can be overridden with the `NGTOP_DB` environment variable.
- The command line arguments express a filtering criteria, used to build the SQL query that counts the requests.
  - For instance, the command `ngtop url -w url=/blog/%` produces:
    ```sql
    SELECT path,count(1) '#reqs' FROM access_logs
    WHERE time > ? AND time < ? AND (path LIKE ?)
    GROUP BY 1
    ORDER BY count(1)
    DESC LIMIT 5
    ```

## Configuration

The command-line arguments and flags are intended exclusively to express a requests count query. The configuration, which isn't expected to change across command invocations, is left to environment variables:

- `NGTOP_LOGS_PATH`: path pattern to find the nginx access logs. Defaults to `/var/log/nginx/access.log*`. The pattern is expanded using Go's [`path/filepath.Glob`](https://pkg.go.dev/path/filepath#Glob).
- `NGTOP_LOG_FORMAT`: The [nginx log_format](https://nginx.org/en/docs/http/ngx_http_log_module.html#log_format "
") specification to parse the log entries. By default combined logs are assumed, which is equivalent to:
  ```
  NGTOP_LOG_FORMAT='$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"'
  ```
- `NGTOP_DEBUG`: when set, internal logs will be printed to standard output.
- `NGTOP_DB`: location of the SQLite db where the parsed logs are stored. Defaults to `./ngtop.db`.
