# zm

CLI for z/OS mainframe operations.

## Features

- **Dataset operations** — list, read, edit, grep, diff members
- **USS operations** — list, read, write files
- **JCL** — submit jobs, check status, view output
- **Three protocols** — z/OSMF, FTP, SSH (transparent switching via config)
- **EBCDIC** — automatic encoding detection and conversion
- **Retry** — configurable retry with exponential backoff

## Installation

Download the latest release from [Releases](https://github.com/AlexFalzone/zm/releases), or build from source:

```bash
git clone https://github.com/AlexFalzone/zm.git
cd zm
go build -o zm .
```

## Configuration

```bash
zm config setup
```

Config file is stored at `~/.zmconfig`:

```yaml
profiles:
  myprofile:
    host: mainframe.example.com
    port: 443
    user: MYUSER
    password: mypassword
    protocol: zosmf       # zosmf, ftp, ssh
    hlq: MYUSER
    uss_home: /u/myuser
    encoding: ""           # "", "ascii", "ebcdic" (empty = auto-detect)
    retry_attempts: 3
    retry_delay: 1s

default_profile: myprofile
```

## Usage
Just use the help section on the CLI.
