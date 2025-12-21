# Go Programming Language Reference

Go is an open-source programming language designed for building simple, reliable, and efficient software.

## Overview

This repository contains the complete source code for the Go compiler, runtime, and standard library. The standard library follows a compositional design where small, focused interfaces (like `io.Reader` and `io.Writer`) compose together to build complex functionality.

### Key Features

- Explicit error handling
- Context-aware operations for cancellation and timeouts
- Built-in support for concurrent programming through goroutines and channels
- 75+ packages with extensive example test files
- Ongoing improvements in security, performance, and usability

## Memory Model

Go approaches its memory model in much the same way as the rest of the language, aiming to keep the semantics simple, understandable, and useful.

## Code Examples

### Simple Hello World Program

```go
package main

import "os"

func main() {
    os.Stdout.WriteString("Hello, world!")
}
```

### Test Project Setup with Examples and Benchmarks

**go.mod:**
```go.mod
module m

go 1.16
```

**foo_test.go:**
```go
package foo

import "testing"

func TestOne(t *testing.T)   {}
func TestTwo(t *testing.T)   {}
func TestThree(t *testing.T) {}

func BenchmarkOne(b *testing.B)   {}
func BenchmarkTwo(b *testing.B)   {}
func BenchmarkThree(b *testing.B) {}
```

### Example Functions with Output Validation

```go
package testlist

import (
    "fmt"
)

func Example_simple() {
    fmt.Println("Test with Output.")
    // Output: Test with Output.
}

func Example_withEmptyOutput() {
    fmt.Println("")
    // Output:
}

func Example_noOutput() {
    _ = fmt.Sprint("Test with no output")
}
```

### Simple Hello Function

```go
package greeterv2

func Hello() string {
    return "hello, world v2"
}
```

## Development Setup

### Setup with toolstash

```bash
go install golang.org/x/tools/cmd/toolstash@latest
git clone https://go.googlesource.com/go
export PATH=$PWD/go/bin:$PATH
cd go/src
git checkout -b mybranch
./all.bash
toolstash save
```

### Go Vendoring

```bash
env GO111MODULE=off
cd vend/hello
go run hello.go
stdout 'hello, world'
```

## Git Commands for Go Projects

### Initialize and Tag Repository

```bash
git init
git add cmd
git commit -m 'add cmd/issue47650'
git branch -m main
git tag v0.1.0
```

### Add Go Module

```bash
git add go.mod
git commit -m 'add go.mod'
```

## Resources

- [Official Go Documentation](https://go.dev/doc/)
- [Go Standard Library](https://pkg.go.dev/std)
- [Effective Go](https://go.dev/doc/effective_go)
