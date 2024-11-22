## RepoArk
A robust Go command-line tool for archiving Git repositories, including both tracked and untracked files (respecting .gitignore).


## Features

Basiclly it does the same thing as the following shell script:


```bash
{ git ls-files --others --exclude-standard --cached && find .git -type f; } | \
sort -u | \
tar --exclude "$(basename $(pwd)).tar.gz" -czvf "$(basename $(pwd)).tar.gz" -T -
```

add some features to overcome the limitations of shell script:

- Support submodule
- Cross-platform native program
- Archive from any directory path


## Installation

### Option 1: Build from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/likang/RepoArk.git
   cd RepoArk
   ```

2. Build the program:
   ```bash
   go build -o repoark
   ```

### Option 2: Go Install

```bash
go install github.com/likang/RepoArk@latest
```

## Usage

```bash
./repoark /path/to/your/git/repository
```


## Contributing

Contributions are welcome! Please:
- Fork the repository
- Create a feature branch
- Submit a pull request

## License

MIT

