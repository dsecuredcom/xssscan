# XSSScan

A fast, concurrent XSS (Cross-Site Scripting) scanner designed to efficiently test multiple URLs and parameters for reflection vulnerabilities.

## Features

- **Fast & Concurrent**: Configurable request rate limiting and worker pools for optimal performance
- **Batch Processing**: Test multiple parameters per request to reduce HTTP overhead
- **Flexible Input**: Support for custom URL lists and parameter wordlists
- **Multiple HTTP Methods**: Support for both GET and POST requests
- **Proxy Support**: Built-in proxy support for testing through tools like Burp Suite
- **Real-time Results**: Immediate reporting of reflections as they're discovered
- **Unique Payloads**: MD5-based payload generation to avoid false positives from cached responses
- **TLS Flexibility**: Option to ignore certificate errors for testing environments

## Installation

```bash
go install github.com/dsecuredcom/xssscan@latest
```

Or clone and build from source:

```bash
git clone https://github.com/dsecuredcom/xssscan.git
cd xssscan
go build -o xssscan .
```

## Usage

### Basic Usage

```bash
xssscan -paths urls.txt -parameters params.txt
```

### Advanced Usage

```bash
xssscan \
  -paths urls.txt \
  -parameters params.txt \
  -method POST \
  -concurrency 50 \
  -parameter-batch 10 \
  -proxy http://127.0.0.1:8080 \
  -verbose
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-paths` | *required* | File containing target URLs (one per line) |
| `-parameters` | *required* | File containing parameter names to test (one per line) |
| `-method` | `GET` | HTTP method to use (`GET` or `POST`) |
| `-concurrency` | `20` | Maximum requests per second |
| `-parameter-batch` | `5` | Number of parameters to test per request |
| `-timeout` | `15s` | HTTP client timeout per request |
| `-proxy` | | Upstream proxy (e.g., `http://127.0.0.1:8080`) |
| `-workers` | `concurrency*2` | Number of worker goroutines |
| `-insecure` | `false` | Ignore TLS certificate errors |
| `-retries` | `0` | Number of retries on request failure |
| `-verbose` | `false` | Show all requests and HTTP status codes |

## Input File Formats

### URLs File (`urls.txt`)
```
https://example.com/search
https://example.com/login
https://api.example.com/v1/users
```

### Parameters File (`params.txt`)
```
q
search
query
username
email
id
callback
```

Comments (lines starting with `#`) and empty lines are ignored in both files.

## How It Works

1. **Payload Generation**: For each parameter, XSSScan generates unique payloads using MD5 hashes to create patterns like `abc">de` and `abc'>de`
2. **Batch Processing**: Parameters are grouped into batches to test multiple parameters per HTTP request
3. **Concurrent Execution**: Multiple workers process requests concurrently while respecting rate limits
4. **Reflection Detection**: Response bodies are checked for payload reflection
5. **Real-time Reporting**: Reflections are reported immediately when discovered

## Example Output

```
[+] Loading input files... Done
[+] Loaded:
    • 50 paths
    • 100 parameters
    • 20 chunks (parameters/chunk size: 100/5)
    • 2000 HTTP requests total (50 paths × 20 chunks × 2 variants)
[+] Starting 20 RPS with 40 workers...
[+] Reflections will be reported immediately as found:

[REFLECTED] [GET] https://example.com/search?q=abc%22%3Ede&filter=test
[REFLECTED] [POST] https://example.com/login
username=abc'>de

[+] Scan completed. Final summary:
⚠️  Total reflections found: 2
Please verify these findings manually.
```

## Proxy Integration

XSSScan automatically enables insecure mode when using a proxy, making it compatible with intercepting proxies like Burp Suite:

```bash
xssscan -paths urls.txt -parameters params.txt -proxy http://127.0.0.1:8080
```

## Performance Tips

- **Batch Size**: Larger batches (10-20) reduce HTTP requests but may miss some reflections if the server has parameter limits
- **Concurrency**: Start with lower values (10-20) and increase based on target server capacity
- **Workers**: Default is `concurrency * 2`, but you can tune this based on your system resources

## Methodology

XSSScan uses a systematic approach to XSS testing:

1. **Unique Payloads**: Each parameter gets a unique payload based on its MD5 hash to avoid caching issues
2. **Dual Variants**: Tests both single (`'`) and double (`"`) quote contexts
3. **Batch Optimization**: Groups parameters to minimize HTTP requests while maintaining coverage
4. **Rate Limiting**: Respects server capacity to avoid overwhelming targets

## Security Considerations

- This tool is intended for authorized security testing only
- Always obtain proper permission before testing systems you don't own
- Be respectful of rate limits and server resources
- Manually verify all reported reflections to confirm exploitability

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.

## License

This project is licensed under the MIT License - see the LICENSE file for details.