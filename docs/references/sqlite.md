# SQLite Reference

SQLite is a C-language library that provides a lightweight, self-contained, serverless, transactional, and fully featured SQL database engine.

## Overview

SQLite is designed with a modular architecture. It's the most widely deployed database engine in the world, used in countless applications from mobile apps to web browsers.

### Key Features

- Self-contained with minimal dependencies
- Serverless - no separate server process
- Zero-configuration
- Transactional (ACID compliant)
- Cross-platform
- Small footprint

## Architecture

For a comprehensive understanding of SQLite's design, refer to:
- Architectural description
- File format documentation
- Virtual machine for prepared statements
- Transaction mechanism
- Query planner overview

## Development Tools

### Lemon Parser Generator

Lemon is a parser generator used in SQLite development.

**Compiling Lemon (Unix):**
```bash
cc -o lemon lemon.c
```

**Compiling Lemon (Windows):**
```bash
cl lemon.c
```

### Grammar Rule Example

```lemon
expr ::= expr PLUS expr.
expr ::= expr TIMES expr.
expr ::= LPAREN expr RPAREN.
expr ::= VALUE.
```

### Parsing a File with Lemon

```c
ParseTree *ParseFile(const char *zFilename){
   Tokenizer *pTokenizer;
   void *pParser;
   Token sToken;
   int hTokenId;
   ParserState sState;

   pTokenizer = TokenizerCreate(zFilename);
   pParser = ParseAlloc( malloc );
   InitParserState(&sState);
   while( GetNextToken(pTokenizer, &hTokenId, &sToken) ){
      Parse(pParser, hTokenId, sToken, &sState);
   }
   Parse(pParser, 0, sToken, &sState);
   ParseFree(pParser, free );
   TokenizerFree(pTokenizer);
   return sState.treeRoot;
}
```

### Parser Usage Pattern

```c
while( GetNextToken(pTokenizer,&hTokenId, &sToken) ){
   Parse(pParser, hTokenId, sToken);
}
Parse(pParser, 0, sToken);
ParseFree(pParser, free );
```

## WebAssembly (WASM) Support

### Install Emscripten SDK (Linux)

```bash
# Clone the emscripten repository:
sudo apt install git
git clone https://github.com/emscripten-core/emsdk.git
cd emsdk

# Download and install the latest SDK tools:
./emsdk install latest

# Make the "latest" SDK "active" for the current user:
./emsdk activate latest
```

### Remote Testing Setup

```bash
# Remote: Install git, emsdk, and althttpd (version 2022-09-26 or newer)
# Remote: Install the SQLite source tree. CD to ext/wasm
# Remote: "make" to build WASM
# Remote: althttpd --enable-sab --port 8080 --popup

# Local: ssh -L 8180:localhost:8080 remote
# Local: Point your web-browser at http://localhost:8180/index.html
```

## Autosetup

### Clone and Update Repository

```bash
git clone https://github.com/msteveb/autosetup
cd autosetup
# Or, if it's already checked out:
git pull
```

### Install and Verify

```bash
/path/to/autosetup-checkout/autosetup --install .
fossil status # show the modified files
```

### Key Autosetup Functions

| Function | Description |
|----------|-------------|
| `file-isexec` | Use instead of `[file executable]` for Windows compatibility |
| `get-env` | Fetches environment variables from configure arguments or system |
| `proj-get-env` | Extends `get-env` by checking for `./.env-$VAR` files |
| `proj-fatal` | Emits a message to stderr and exits |
| `proj-if-opt-truthy` | Evaluates scripts based on a flag's truthiness |
| `proj-indented-notice` | Emits indented messages for important notices |
| `proj-opt-truthy` | Checks if a flag's value is truthy |
| `proj-opt-was-provided` | Checks if a flag was explicitly given to configure |

## JSONB Encoding

SQLite headers can encode JSON values in different ways. Example encodings for the numeric value '1':

```plaintext
0x13 0x31
0xc3 0x01 0x31
0xd3 0x00 0x01 0x31
0xe3 0x00 0x00 0x00 0x01 0x31
0xf3 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x01 0x31
```

## Security

### Trusted Schema

Application-defined functions and virtual tables are classified as Normal by default unless the application explicitly changes their risk level.

For optimal security:
- Risky application-defined functions should be marked as Direct-Only
- TRUSTED_SCHEMA defaults to on for backwards compatibility
- Applications should turn it off when appropriate

## Extensions

The `ext/misc` folder contains smaller loadable extensions for SQLite. Each extension is implemented in a single file of C code.

## JavaScript Bindings (jaccwabyt)

### Defining Get Adaptors

```javascript
// Define an adaptor:
structBinder.adaptGet("to-js-string", (value) => value.toString());

// Struct description using the adaptor:
const structDescription = {
  fields: [
    { name: "myField", type: "uint32", adaptGet: "to-js-string" }
  ]
};
```

## Performance Testing

For performance and size comparisons between SQLite versions, you will need:
- fossil
- valgrind
- tclsh
- An SQLite source tree

## Resources

- [SQLite Official Documentation](https://www.sqlite.org/docs.html)
- [SQLite Command Line Shell](https://www.sqlite.org/cli.html)
- [SQLite WebAssembly](https://sqlite.org/wasm/doc/trunk/index.md)
- [SQLite Source Code](https://github.com/sqlite/sqlite)
