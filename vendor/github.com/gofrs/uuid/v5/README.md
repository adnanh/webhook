# UUID

[![License](https://img.shields.io/github/license/gofrs/uuid.svg)](https://github.com/gofrs/uuid/blob/master/LICENSE)
[![Build Status](https://travis-ci.org/gofrs/uuid.svg?branch=master)](https://travis-ci.org/gofrs/uuid)
[![GoDoc](http://godoc.org/github.com/gofrs/uuid?status.svg)](http://godoc.org/github.com/gofrs/uuid)
[![Coverage Status](https://codecov.io/gh/gofrs/uuid/branch/master/graphs/badge.svg?branch=master)](https://codecov.io/gh/gofrs/uuid/)
[![Go Report Card](https://goreportcard.com/badge/github.com/gofrs/uuid)](https://goreportcard.com/report/github.com/gofrs/uuid)

Package uuid provides a pure Go implementation of Universally Unique Identifiers
(UUID) variant as defined in RFC-4122. This package supports both the creation
and parsing of UUIDs in different formats.

This package supports the following UUID versions:
* Version 1, based on timestamp and MAC address (RFC-4122)
* Version 3, based on MD5 hashing of a named value (RFC-4122)
* Version 4, based on random numbers (RFC-4122)
* Version 5, based on SHA-1 hashing of a named value (RFC-4122)

This package also supports experimental Universally Unique Identifier implementations based on a
[draft RFC](https://www.ietf.org/archive/id/draft-peabody-dispatch-new-uuid-format-04.html) that updates RFC-4122
* Version 6, a k-sortable id based on timestamp, and field-compatible with v1 (draft-peabody-dispatch-new-uuid-format, RFC-4122)
* Version 7, a k-sortable id based on timestamp (draft-peabody-dispatch-new-uuid-format, RFC-4122)

The v6 and v7 IDs are **not** considered a part of the stable API, and may be subject to behavior or API changes as part of minor releases
to this package. They will be updated as the draft RFC changes, and will become stable if and when the draft RFC is accepted.

## Project History

This project was originally forked from the
[github.com/satori/go.uuid](https://github.com/satori/go.uuid) repository after
it appeared to be no longer maintained, while exhibiting [critical
flaws](https://github.com/satori/go.uuid/issues/73). We have decided to take
over this project to ensure it receives regular maintenance for the benefit of
the larger Go community.

We'd like to thank Maxim Bublis for his hard work on the original iteration of
the package.

## License

This source code of this package is released under the MIT License. Please see
the [LICENSE](https://github.com/gofrs/uuid/blob/master/LICENSE) for the full
content of the license.

## Recommended Package Version

We recommend using v2.0.0+ of this package, as versions prior to 2.0.0 were
created before our fork of the original package and have some known
deficiencies.

## Requirements

This package requres Go 1.17 or later

## Usage

Here is a quick overview of how to use this package. For more detailed
documentation, please see the [GoDoc Page](http://godoc.org/github.com/gofrs/uuid).

```go
package main

import (
	"log"

	"github.com/gofrs/uuid"
)

// Create a Version 4 UUID, panicking on error.
// Use this form to initialize package-level variables.
var u1 = uuid.Must(uuid.NewV4())

func main() {
	// Create a Version 4 UUID.
	u2, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("failed to generate UUID: %v", err)
	}
	log.Printf("generated Version 4 UUID %v", u2)

	// Parse a UUID from a string.
	s := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	u3, err := uuid.FromString(s)
	if err != nil {
		log.Fatalf("failed to parse UUID %q: %v", s, err)
	}
	log.Printf("successfully parsed UUID %v", u3)
}
```

## References

* [RFC-4122](https://tools.ietf.org/html/rfc4122)
* [DCE 1.1: Authentication and Security Services](http://pubs.opengroup.org/onlinepubs/9696989899/chap5.htm#tagcjh_08_02_01_01)
* [New UUID Formats RFC Draft (Peabody) Rev 04](https://www.ietf.org/archive/id/draft-peabody-dispatch-new-uuid-format-04.html#)
