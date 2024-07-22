# ngtop

TODO what is ntop

TODO screenshot with fancy output

## Installation

TODO

## Usage examples

Count requests from the last hour:

    $ ngtop --since 1h
    $ ngtop -s 1h
    $ ngtop

Count requests from the last minute, day, or month:

    $ ngtop -s 1m
    $ ngtop -s 1d
    $ ngtop -s 1M

Count requests from the day before:

    $ ngtop --since 2d --until 1d
    $ ngtop -s 2d -u 1d

Show the top 5 urls in the last hour:

    $ ngtop url
    $ ngtop path

Show the top 10 urls in the last minute:

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

Show the top urls by user agent pattern:

    $ ngtop url -w ua=Mozilla/%
    $ ngtop url -w ua=%iPhone%

Show the top referers for a url pattern:

    $ ngtop referer -w url=/blog/%

Show the top user agent and referer combination

    $ ngtop ua referer

Show the top user agent and referer combination for a specific url

    $ ngtop ua referer -w url=/blog/code-is-run-more-than-read

Count total 404 status responses:

    $ ngtop -w status=404

Count total 404 error responses:

    $ ngtop -w status=4% - status=5%

## Configuration

## How it works
