# zm

CLI tool for interacting with z/OS mainframes via FTP (at least for now). Lightweight alternative to Zowe.

## Installation

```bash
go install github.com/AlexFalzone/zm@latest
```

Or build from source:

```bash
git clone https://github.com/AlexFalzone/zm.git
cd zm
make build
```

## Configuration

Create a profile:

```bash
zm config setup
```

Config file is stored at `~/.zmconfig`:

```yaml
profiles:
  default:
    host: mainframe.example.com
    port: 21
    user: MYUSER
    password: mypassword
    hlq: MYUSER
    uss_home: /u/myuser

default_profile: default
```

## Commands

Just use the help section on the CLI.
