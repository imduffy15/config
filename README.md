# Config
[![Documentation](https://godoc.org/github.com/hookactions/config?status.svg)](http://godoc.org/github.com/hookactions/config)
[![CircleCI](https://circleci.com/gh/hookactions/config.svg?style=svg)](https://circleci.com/gh/hookactions/config)
[![Go Report Card](https://goreportcard.com/badge/github.com/hookactions/config)](https://goreportcard.com/report/github.com/hookactions/config)
[![license](https://img.shields.io/github/license/hookactions/config.svg?maxAge=2592000)](https://github.com/hookactions/config/LICENSE)
[![Release](https://img.shields.io/github/release/hookactions/config.svg?label=Release)](https://github.com/hookactions/config/releases)

Manage your application config as a typesafe struct in as little as two function calls.

```go
package main

import (
	"context"
	"fmt"
	
	"github.com/hookactions/config"
)

type MyConfig struct {
	DatabaseUrl string `config:"DATABASE_URL"`
	FeatureFlag bool   `config:"FEATURE_FLAG"`
	Port        int // tags are optional. PORT is assumed
	...
}

var c MyConfig
config.FromEnv().To(&c)

fmt.Printf("%v\n", c)

// Supports AWS Secret Manager and Parameter store
// sm://my_value
// ssm://my_value

p, _ := config.NewAWSSecretManagerValuePreProcessor(context.Background(), true)
config.WithValuePreProcessor(p).FromEnv().To(&c)

fmt.Printf("%v\n", c)
```

## How It Works

Its just simple, pure stdlib with optional AWS support. 

* A field's type determines what [strconv](https://golang.org/pkg/strconv/) function is called.
* All string conversion rules are as defined in the [strconv](https://golang.org/pkg/strconv/) package
* If chaining multiple data sources, data sets are merged. 
  Later values override previous values.
  ```go
  config.From("dev.config").FromEnv().To(&c)
  ```
    
* Unset values remain as their native [zero value](https://tour.golang.org/basics/12) 
* Nested structs/subconfigs are delimited with double underscore 
    * e.g. `PARENT__CHILD`
* Env vars map to struct fields case insensitively
    * NOTE: Also true when using struct tags.

## Why you should use this

* Its the cloud-native way to manage config. See [12 Factor Apps](https://12factor.net/config)
* Simple:
    * only 2 lines to configure.
* Composeable:
    * Merge local files and environment variables for effortless local development.
    
## Design Philosophy

Opinionated and narrow in scope. This library is only meant to do config binding. 
Feel free to use it on its own, or alongside other libraries.  

* Only structs at the entry point. This keeps the API surface small.  

* Slices are space delimited. This matches how environment variables and commandline args are handled by the `go` cmd.

* No slices of structs. The extra complexity isn't warranted for such a niche usecase.

* No maps. The only feature of maps not handled by structs for this usecase is dynamic keys.

* No pointer members. If you really need one, just take the address of parts of your struct.
