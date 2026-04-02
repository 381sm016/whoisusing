# whoisusing

A tiny, fast CLI tool that tells you what process is using a port.

No dependencies. Single binary. Works on Windows, Linux, and macOS.

## Install

### From source (requires Go 1.21+)

```bash
go install github.com/381sm016/whoisusing@latest
```

### Download binary

Grab the latest release from [Releases](../../releases).

## Usage

```bash
# Check what's using port 8080
whoisusing 8080

# List all listening ports
whoisusing --all

# Output
PROTO  PORT  PID    PROCESS
TCP    8080  12345  node.exe
```

### Flags

| Flag | Description |
|------|-------------|
| `<port>` | Show process using a specific port |
| `--all`, `-a` | List all listening ports and their processes |
| `--help`, `-h` | Show help |
| `--version`, `-v` | Show version |

## Build from source

```bash
git clone https://github.com/381sm016/whoisusing.git
cd whoisusing
go build -o whoisusing .
```

## Cross-compile

Go makes it easy to build for any platform:

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o whoisusing .

# macOS
GOOS=darwin GOARCH=arm64 go build -o whoisusing .

# Windows
GOOS=windows GOARCH=amd64 go build -o whoisusing.exe .
```

## License

MIT
