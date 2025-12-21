# Gin Web Framework Reference

Gin is a high-performance HTTP web framework for Go, featuring a martini-like API with superior speed for building REST APIs, web applications, and microservices.

## Overview

### Key Features

- **Zero allocation router**: Extremely memory-efficient routing with no heap allocations
- **High performance**: Benchmarks show superior speed compared to other Go web frameworks
- **Middleware support**: Extensible middleware system for authentication, logging, CORS, etc.
- **Crash-free**: Built-in recovery middleware prevents panics from crashing your server
- **JSON validation**: Automatic request/response JSON binding and validation
- **Route grouping**: Organize related routes and apply common middleware
- **Error management**: Centralized error handling and logging
- **Built-in rendering**: Support for JSON, XML, HTML templates, and more
- **Extensible**: Large ecosystem of community middleware and plugins

## Code Examples

### Basic HTTP Methods

```go
func main() {
    // Creates a gin router with default middleware:
    // logger and recovery (crash-free) middleware
    router := gin.Default()

    router.GET("/someGet", getting)
    router.POST("/somePost", posting)
    router.PUT("/somePut", putting)
    router.DELETE("/someDelete", deleting)
    router.PATCH("/somePatch", patching)
    router.HEAD("/someHead", head)
    router.OPTIONS("/someOptions", options)

    // By default, it serves on :8080 unless a
    // PORT environment variable was defined.
    router.Run()
    // router.Run(":3000") for a hard coded port
}
```

### Query String Parameters

```go
func main() {
    router := gin.Default()

    // The request responds to a URL matching: /welcome?firstname=Jane&lastname=Doe
    router.GET("/welcome", func(c *gin.Context) {
        firstname := c.DefaultQuery("firstname", "Guest")
        lastname := c.Query("lastname") // shortcut for c.Request.URL.Query().Get("lastname")

        c.String(http.StatusOK, "Hello %s %s", firstname, lastname)
    })
    router.Run(":8080")
}
```

### Serving Static Files

```go
func main() {
    router := gin.Default()
    router.Static("/assets", "./assets")
    router.StaticFS("/more_static", http.Dir("my_file_system"))
    router.StaticFile("/favicon.ico", "./resources/favicon.ico")
    router.StaticFileFS("/more_favicon.ico", "more_favicon.ico", http.Dir("my_file_system"))

    router.Run(":8080")
}
```

### HTML Rendering with Multiple Templates

```go
func main() {
    router := gin.Default()
    router.LoadHTMLGlob("templates/**/*")

    router.GET("/posts/index", func(c *gin.Context) {
        c.HTML(http.StatusOK, "posts/index.tmpl", gin.H{
            "title": "Posts",
        })
    })

    router.GET("/users/index", func(c *gin.Context) {
        c.HTML(http.StatusOK, "users/index.tmpl", gin.H{
            "title": "Users",
        })
    })

    router.Run(":8080")
}
```

### HTTP Redirects

```go
// External redirect
r.GET("/test", func(c *gin.Context) {
    c.Redirect(http.StatusMovedPermanently, "http://www.google.com/")
})

// Internal redirect from POST
r.POST("/test", func(c *gin.Context) {
    c.Redirect(http.StatusFound, "/foo")
})

// Router redirect
r.GET("/test", func(c *gin.Context) {
    c.Request.URL.Path = "/test2"
    r.HandleContext(c)
})
r.GET("/test2", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"hello": "world"})
})
```

### Single Binary with Embedded Templates

```go
package main

import (
    "embed"
    "html/template"
    "net/http"

    "github.com/gin-gonic/gin"
)

//go:embed assets/* templates/*
var f embed.FS

func main() {
    router := gin.Default()
    templ := template.Must(template.New("").ParseFS(f, "templates/*.tmpl", "templates/foo/*.tmpl"))
    router.SetHTMLTemplate(templ)

    // example: /public/assets/images/example.png
    router.StaticFS("/public", http.FS(f))

    router.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "index.tmpl", gin.H{
            "title": "Main website",
        })
    })

    router.GET("/foo", func(c *gin.Context) {
        c.HTML(http.StatusOK, "bar.tmpl", gin.H{
            "title": "Foo website",
        })
    })

    router.GET("favicon.ico", func(c *gin.Context) {
        file, _ := f.ReadFile("assets/favicon.ico")
        c.Data(
            http.StatusOK,
            "image/x-icon",
            file,
        )
    })

    router.Run(":8080")
}
```

## API Reference

### Serving Data from File

| Method | Description |
|--------|-------------|
| `c.File(filepath string)` | Serves a file from the local filesystem |
| `c.FileFromFS(name string, fs http.FileSystem)` | Serves a file from an `http.FileSystem` |

### Static File Methods

| Method | Description |
|--------|-------------|
| `router.Static(relativePath, root string)` | Serves files from a directory |
| `router.StaticFS(relativePath, fs http.FileSystem)` | Serves files from an `http.FileSystem` |
| `router.StaticFile(relativePath, filepath string)` | Serves a single file |
| `router.StaticFileFS(relativePath, name string, fs http.FileSystem)` | Serves a single file from an `http.FileSystem` |

### Request Binding

The `ShouldBind` method attempts to bind request data from the query string (for GET requests) or from the request body (for POST requests).

**Supported Parameters:**
- `name` (string) - The name of the person
- `address` (string) - The address
- `birthday` (time.Time) - Expected in 'YYYY-MM-DD' format
- `createTime` (time.Time) - Creation time in nanoseconds since epoch
- `unixTime` (time.Time) - Timestamp in seconds since epoch

**Example Request:**
```bash
curl -X GET "localhost:8085/testing?name=appleboy&address=xyz&birthday=1992-03-15&createTime=1562400033000000123"
```

## Resources

- [Gin Documentation](https://gin-gonic.com/)
- [Gin Quick Start Guide](https://github.com/gin-gonic/gin/blob/master/docs/doc.md)
- [Gin Contrib (Official Middleware)](https://github.com/gin-contrib)
- [Multitemplate Render](https://github.com/gin-contrib/multitemplate)
