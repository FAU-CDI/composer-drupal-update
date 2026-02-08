# composer-drupal-update

[![CI](https://github.com/FAU-CDI/composer-drupal-update/actions/workflows/ci.yml/badge.svg)](https://github.com/FAU-CDI/composer-drupal-update/actions/workflows/ci.yml)
[![WTFPL](https://img.shields.io/badge/license-WTFPL-blue.svg)](http://www.wtfpl.net/)

> [!WARNING]
> This project is (almost) entirely vibe coded. It is shared in the hope that it is useful, but comes with absolutely no warranty whatsoever. Use at your own risk.

A tool for interactively updating version constraints in `composer.json` files in a drupal context. It queries [drupal.org](https://www.drupal.org/) for Drupal module releases and [Packagist](https://packagist.org/) for all other packages, letting you pick new versions without invoking Composer itself.

## Components

- **Go library** (`drupalupdate` package) — core logic for parsing `composer.json`, fetching releases from drupal.org and Packagist, and rewriting version constraints.
- **CLI tool** (`cmd/composer-drupal-update`) — interactive terminal tool that walks you through each package and lets you select a version.
- **Web server** (`cmd/composer-drupal-server`) — HTTP server that exposes a JSON API, an embedded frontend, and Swagger UI documentation.
- **Frontend** (`frontend/`) — plain JavaScript single-page app with drag-and-drop `composer.json` loading, a version-selection table, and copyable Composer commands.

## Running

### Web UI (recommended)

```
go run ./cmd/composer-drupal-server -addr :8080
```

Then open http://localhost:8080 in your browser.


### CLI

```
go run ./cmd/composer-drupal-update path/to/composer.json
```

Then open:

| URL | Description |
|---|---|
| `http://localhost:8080/` | Frontend app |
| `http://localhost:8080/api/` | API |
| `http://localhost:8080/doc/` | Swagger UI |
| `http://localhost:8080/openapi.yaml` | OpenAPI spec |

### Tests

```
go test ./...
cd frontend && yarn test
cd frontend && yarn typecheck
```

## License

[WTFPL](http://www.wtfpl.net/) — Do What The Fuck You Want To Public License. See [LICENSE](LICENSE).
