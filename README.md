# disturbancesmlx [![Discord](https://img.shields.io/discord/334423823552217090.svg)](https://discord.gg/hhuC7fc) [![license](https://img.shields.io/github/license/gbl08ma/disturbancesmlx.svg)](https://github.com/underlx/disturbancesmlx/blob/master/LICENSE) [![CI status](https://travis-ci.org/underlx/disturbancesmlx.svg?branch=master)](https://travis-ci.org/underlx/disturbancesmlx)
Server for the UnderLX app, providing information about public transit networks and handling user-contributed information. It is also responsible for serving a fancy status page for the [Lisbon Metro](http://www.metrolisboa.pt/), scraping the official website for some of the information it shows. Live at https://perturbacoes.pt.

The server is written in Go. It is compatible with PostgreSQL only and designed to run behind a reverse proxy (to handle e.g. HTTPS), although it does not need one for experimentation/development.

It is prepared to work with other transit networks, including multiple networks at once, and multiple sources of information for a single network. It supports mixing official sources of information, like service status pages, with unofficial sources, like user reports, managing the lifecycle of service disruptions ("disturbances") accordingly. A modular scraper architecture has been designed to this effect.

It can also be extended to support other kinds of service disruption events ("disturbances"), including whole-network incidents. Right now all "disturbances" are line-oriented.

Since the service, in its current form, monitors a Portuguese subway network, it targets a Portuguese audience and the website is in pt-PT. All code and comments are in English. The REST API provided by this server is locale-agnostic and provides to its clients all the information required for correct localization, such as the timezone of the transport networks (so that timetables, for instance, can be correcly displayed and computed upon).

The website contains a heavily modified version of [cnanney's CSS flip counter](https://github.com/cnanney/css-flip-counter).

## Installation

We assume you already have a [Go](https://golang.org/) development environment set up. This project uses [dep](https://golang.github.io/dep/) for dependency management. Begin by [installing dep](https://golang.github.io/dep/docs/installation.html) (it's easy).

You should then clone this repo with `go get -u github.com/underlx/disturbancesmlx`, then use `dep ensure` to download and install the right versions of the dependencies in the vendor directory.

Create a new PostgreSQL database and use the `schema.sql` file to create its schema. Edit the database connection string in `secrets-debug.json`.

`go build`, run `disturbancesmlx` and wait for the "Scraper completed second fetch" log message to appear. The HTTP server should be available on localhost port 8089 by then.

## Contributing

Contributions are welcome. Fork this project, commit your changes and open a pull request.

## Disclaimer

I have no affiliation with _Metropolitano de Lisboa, E.P.E._. The code and the associated website are not sponsored or endorsed by them. I shall not be liable for any damages arising from the use of this code or associated website.
