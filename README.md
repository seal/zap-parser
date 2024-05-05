[![Documentation](https://godoc.org/github.com/Yacast/zap-parser?status.svg)](https://godoc.org/github.com/Yacast/zap-parser)

# zap-parser

A golang parser for [Uber's zap](https://github.com/uber-go/zap) logger json output.

## Quick Start

### Times:

If your logs are in the timestamp format:

```
"2006-01-02T15:04:05.999Z"
```

Please use p.TimeIsNonUnix


If your times are in 

```
1568146267.0402062
```

format, please change nothing
```go
package main

import (
	"fmt"

	zapparser "github.com/seal/zap-parser"
)

func main() {
	p, err := zapparser.FromFile("./logs.log")
	p.TimeIsNonUnix = true
	if err != nil {
		panic(err)
	}

	p.OnError(func(err error) {
		fmt.Println(err)
	})

	p.OnEntry(func(e *zapparser.Entry) {
		fmt.Println(e.Message)
	})

	p.Start()
	fmt.Println("Done parsing...")

}
```
