# Jex - The Lightweight JSON Explorer

Jex is a simple tool for navigating and querying JSON data. Jex provides an interface with query searching and dynamic previews to help you efficiently explore complex JSON structures.

<img src="https://raw.githubusercontent.com/jedipunkz/jex/main/static/pix/jex.gif">


## Installation

Make sure you have [Go](https://golang.org/) installed on your system.

```bash
go install github.com/jedipunkz/jex@latest
```

or Download binary from Release Page.
https://github.com/jedipunkz/jex/releases

## Usage

Run Jex with a JSON file as input:

```bash
jex <JSON_FILE>
```

or jex can recieve input from a pipe.

```bash
cat <JSON_FILE> | jex
```

## Author
Jex was created with by jedipunkz.

## License
This project is licensed under the MIT License.
