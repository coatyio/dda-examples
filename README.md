# Code Examples for Data Distribution Agent

[![Powered by DDA](https://img.shields.io/badge/Powered%20by-DDA-00ADD8.svg)](https://github.com/coatyio/dda)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/coatyio/dda-examples/blob/main/LICENSE)

## Table of Contents

* [Overview](#overview)
* [Contributing](#contributing)
* [License](#license)

## Overview

This repository provides a collection of fully documented ready-to-run best
practice examples demonstrating how to use the Data Distribution Agent (DDA) as
a sidecar or library in a decentralized desktop or web application:

* `compute` - a pure Go application with distributed components utilizing DDA
  communication service as a library and as a co-located sidecar over gRPC
* `light-control` - a distributed web app written in Angular/JavaScript/HTML
  connecting to its co-located DDA sidecar over gRPC-Web

For detailed documentation take a look at the README of the individual example
projects and delve into the [DDA Developer
Guide](https://coatyio.github.io/dda/DEVGUIDE.html).

## Contributing

Contributions to DDA examples are welcome and appreciated. Please follow the
recommended practice for idiomatic Go
[programming](https://go.dev/doc/effective_go) and
[documentation](https://tip.golang.org/doc/comment).

## License

Code and documentation copyright 2023 Siemens AG.

Code is licensed under the [MIT License](https://opensource.org/licenses/MIT).

Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
