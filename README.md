# disturbancesmlx [![Discord](https://img.shields.io/discord/334423823552217090.svg)](https://discord.gg/hhuC7fc) [![license](https://img.shields.io/github/license/gbl08ma/disturbancesmlx.svg)](https://github.com/underlx/disturbancesmlx/blob/master/LICENSE)
Fancy status page and status logger for the [Lisbon Metro](http://www.metrolisboa.pt/), that scrapes the official website for information. Live at https://perturbacoes.pt.

The server is written in Go. It is compatible with PostgreSQL only and designed to run behind a reverse proxy.

The code can be modified to work with other networks, including multiple networks at once and multiple sources of information for a single network.

It can also be extended to support other kinds of network incidents ("disturbances"), including whole-network incidents. Right now all "disturbances" are line-oriented.

Since the service, in its current form, monitors a Portuguese subway network, it targets a Portuguese audience and the frontend is in pt-PT. All code and comments are in English.

The website contains a heavily modified version of [cnanney's CSS flip counter](https://github.com/cnanney/css-flip-counter).

## Installation

`go get -u github.com/underlx/disturbancesmlx`, as is tradition with Go projects.

Use the `schema.sql` file to create the schema on your PostgreSQL database, and edit the database connection string in `secrets-debug.json`.

`go build`, run `disturbancesmlx` and wait for the "Scraper completed second fetch" log message to appear. The HTTP server should be available on localhost port 8089 by then.

## Contributing

Contributions are welcome. Fork this project, commit your changes and open a pull request.

## Disclaimer

I have no affiliation with _Metropolitano de Lisboa, E.P.E._. The code and the associated website are not sponsored or endorsed by them. I shall not be liable for any damages arising from the use of this code or associated website.
