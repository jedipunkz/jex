# Jex - The Lightweight JSON Explorer

Jex is a simple tool for navigating and querying JSON data. Designed to enhance productivity, Jex provides an intuitive interface with fuzzy searching and dynamic previews to help you efficiently explore complex JSON structures.

<img src="https://raw.githubusercontent.com/jedipunkz/jex/main/static/pix/jex.gif">


## Installation

Make sure you have [Go](https://golang.org/) installed on your system.

```bash
go install github.com/jedipunkz/jex@latest
```

## Usage

Run Jex with a JSON file as input:

```bash
jex <JSON_FILE>
```

or jex can recieve input from a pipe.

```bash
cat <JSON_FILE> | jex
```


## Example

Given a JSON file data.json:

```json
{
  "name": "Project A",
  "contributors": [
    {"name": "Alice", "email": "alice@example.com"},
    {"name": "Bob", "email": "bob@example.com"}
  ],
  "version": 1.0,
  "isActive": true
}
```

Start exploring the file with:

```bash
jex data.json
```

Interactive Features

- Dynamic Parsing: As you navigate the JSON structure, the parsed result is displayed in real time.
- Fuzzy Search: Type partial keywords to quickly locate keys or paths in deeply nested JSON.
- Colorized Output: Results are displayed in a Dracula-themed color scheme for better readability.

For example, selecting contributors[].name in the tool will display:

```json
["Alice", "Bob"]
```

## Author
Jex was created with by jedipunkz.

## License
This project is licensed under the MIT License.
