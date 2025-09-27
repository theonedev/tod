# TOD - TheOneDev CLI Tool

TOD (**T**he**O**ne**D**ev) is a powerful command-line tool for OneDev 13+ that streamlines your development workflow by enabling you to run CI/CD jobs against local changes, set up local working directory to work on pull requests, etc. It also provides a MCP (Model Context Protocol) server, allowing you to work with OneDev in an intelligent, natural way via AI assistants.

## Features

- **MCP (Model Context Protocol) server** for AI tool integration
- **Run CI/CD jobs against local changes** without committing/pushing
- **Run jobs against specific branches or tags**
- **Checkout pull requests** locally
- **Check and migrate build specifications** to the latest version
- **Real-time log streaming** to console from job execution
- **Configuration management** via config files
- **Cross-platform support** (Windows, macOS, Linux)

## Installation

To install tod, just put tod binary into your PATH. 

### Download Pre-built Binaries

https://code.onedev.io/onedev/tod/~builds?query=%22Job%22+is+%22Release%22+and+successful

### Build Binary from Source

**Requirements:**
- Go 1.22.1 or higher

**Steps:**
1. Clone the repository:
   ```bash
   git clone https://code.onedev.io/onedev/tod.git
   cd tod
   ```

2. Build the binary:
   ```bash
   go build
   ```

## Configuration

TOD uses a configuration file to store commonly used settings, eliminating the need to specify them repeatedly.

### Config File Location

Create a config file at: `$HOME/.todconfig`

### Config File Format

The configuration uses INI format:

```ini
server-url=https://onedev.example.com
access-token=your-personal-access-token
```

## Commands

### `mcp` - Start MCP Server

Start the Model Context Protocol server for AI tool integration.

**Syntax:**
```bash
tod mcp [OPTIONS]
```

**Options:**
- `--log-file <file>` - Specify log file path for debug logging

**Example:**
```bash
# Start MCP server
tod mcp

# Start with debug logging
tod mcp --log-file /tmp/tod-mcp.log
```

**For detailed information about available MCP tools and their parameters, see [MCP Documentation](mcp.md).**


### `run-local` - Run Jobs Against Local Changes

Run CI/CD jobs against your uncommitted local changes without the commit/push/run/check loop.

**Syntax:**
```bash
tod run-local [OPTIONS] <job-name>
```

**Options:**
- `--working-dir <dir>` - Specify working directory (defaults to current directory). Working directory is expected to be inside
   a git repository, with one of the remote pointing to a OneDev project
- `--param <key=value>` or `-p <key=value>` - Specify job parameters (can be used multiple times)

**Examples:**
```bash
# Basic usage
tod run-local ci

# With parameters
tod run-local -p database=postgres -p environment=test ci

# Specify working directory
tod run-local --working-dir /path/to/project ci
```

**How it works:**
1. Stashes your local changes
2. Creates a temporary commit
3. Pushes to a temporal ref on the server
4. Runs the specified job
5. Streams logs back to your terminal
6. Cancels the job if you press Ctrl+C

### `run` - Run Jobs Against Branches or Tags

Run CI/CD jobs against specific branches or tags in the repository.

**Syntax:**
```bash
tod run [OPTIONS] <job-name>
```

**Options:**
- `--working-dir <dir>` - Specify working directory (defaults to current directory). Working directory is expected to be inside
   a git repository, with one of the remote pointing to a OneDev project
- `--branch <branch>` - Run against specific branch (mutually exclusive with --tag)
- `--tag <tag>` - Run against specific tag (mutually exclusive with --branch)
- `--param <key=value>` or `-p <key=value>` - Specify job parameters (can be used multiple times)

**Examples:**
```bash
# Run against main branch
tod run --branch main ci

# Run against a tag
tod run --tag v1.2.3 release

# Run with parameters
tod run --branch develop -p environment=staging ci
```

### `checkout` - Checkout Pull Requests

Checkout pull requests locally for testing and review.

**Syntax:**
```bash
tod checkout [OPTIONS] <pull-request-reference>
```

**Options:**
- `--working-dir <dir>` - Specify working directory (defaults to current directory). Working directory is expected to be inside
   a git repository, with one of the remote pointing to a OneDev project

**Example:**
```bash
# Checkout pull request #123
tod checkout 123

# Checkout in specific directory
tod checkout --working-dir /path/to/project 456
```

### `check-build-spec` - Check and Migrate Build Specifications

Check your `.onedev-buildspec.yml` file for validity and migrate it to the latest version if needed.

**Syntax:**
```bash
tod check-build-spec [OPTIONS]
```

**Options:**
- `--working-dir <dir>` - Directory containing build spec file (defaults to current directory). Working directory is expected to be inside a git repository, with one of the remote pointing to a OneDev project

**Example:**
```bash
# Check build spec in current directory
tod check-build-spec

# Check build spec in specific directory
tod check-build-spec --working-dir /path/to/project
```

## Usage Examples

### Complete Workflow Example

1. **Set up configuration:**
   ```bash
   # Create ~/.todconfig
   echo "server-url=https://onedev.example.com" > ~/.todconfig
   echo "access-token=your-token-here" >> ~/.todconfig
   ```

2. **Test local changes:**
   ```bash
   # Run CI against your uncommitted changes
   cd /path/to/onedev-git-repository
   tod run-local ci
   ```

3. **Run against specific branch:**
   ```bash
   # Run ci job against the main branch
   cd /path/to/onedev-git-repository
   tod run --branch main ci
   ```

4. **Checkout a pull request:**
   ```bash
   # Checkout pull request #123 
   cd /path/to/onedev-git-repository
   tod checkout #123
   ```

### Parameter Usage

```bash
# Multiple parameters of the same key
tod run-local -p env=test -p env=staging -p db=postgres ci
```

## Important Notes to Run Local Job

### Nginx Configuration

If OneDev is running behind Nginx, configure it to disable HTTP buffering for real-time log streaming:

```nginx
location /~api/streaming {
    proxy_pass http://localhost:6610/~api/streaming;
    proxy_buffering off;
}
```

See [OneDev Nginx setup documentation](https://docs.onedev.io/administration-guide/reverse-proxy-setup#nginx) for details.

### Security Considerations 

If the job accesses job secrets. Make sure the authorization field is cleared to allow all jobs. Set authorization to allow all branches is not sufficient as local change will be pushed to a temporal ref not belonging to any branch

### Performance Tips

1. **Large repositories**: Use appropriate clone depth in checkout steps instead of full history
2. **External dependencies**: Implement [caching](https://docs.onedev.io/tutorials/cicd/job-cache) for downloads and intermediate files
3. **Build optimization**: Cache slow-to-generate intermediate files

## Contributing

TOD is part of the OneDev ecosystem. For contributions, issues, and feature requests, visit the [OneDev project](https://code.onedev.io/onedev/tod).

## License

See [license.txt](license.txt) for license information.