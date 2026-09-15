# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest  | :white_check_mark: |
| < latest| :x:                |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability in CTX, please report it responsibly:

1. **Email**: Send a detailed report to **ctx-security@proton.me**
2. **Include**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### What to expect

- **Acknowledgment**: We will acknowledge your report within **48 hours**
- **Assessment**: We will assess the severity and impact within **5 business days**
- **Fix timeline**: Critical vulnerabilities will be patched within **7 days**; others within **30 days**
- **Disclosure**: We will coordinate disclosure timing with you

### Scope

The following are in scope for security reports:

- **Privacy leaks**: Context extraction exposing secrets despite `.ctxignore` or sanitization rules
- **Authentication bypass**: Unauthorized access to shared context
- **Injection attacks**: Malicious content in context files that could affect downstream consumers
- **MCP protocol vulnerabilities**: Exploits via the MCP stdio or HTTP interface
- **Credential exposure**: Token storage, credential handling issues

### Out of scope

- Denial of service on the local CLI tool
- Issues in third-party dependencies (report upstream, but let us know)
- Social engineering attacks

## Security Best Practices for Users

1. **Review `.ctxignore`** — Ensure sensitive directories are excluded
2. **Enable `redact_values = true`** — This is the default; don't disable it
3. **Use `hash_identifiers`** for extra-sensitive projects
4. **Bind MCP server to localhost** — Never expose `ctx serve` to `0.0.0.0` in production
5. **Rotate API tokens** — If using cloud sync, rotate tokens regularly
6. **Review context before sharing** — Use `ctx status` to inspect what's extracted

## Hall of Fame

We gratefully acknowledge security researchers who help keep CTX safe. Contributors will be listed here (with permission).
