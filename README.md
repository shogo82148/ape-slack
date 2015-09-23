# ape-slack

Slack porting of IRC reaction bot framework [ape](https://github.com/m0t0k1ch1/ape)

## Example

``` go
package main

import (
  "log"
  "strings"

  "github.com/shogo82148/ape-slack"
)

func main() {
  con := ape.NewConnection("YOUR API TOKEN")

  con.RegisterChannel("#general")

  con.AddAction("piyo", func(e *ape.Event) {
    con.SendMessage("poyo")
  })

  con.AddAction("say", func(e *ape.Event) {
    con.SendMessage(strings.Join(e.Command().Args(), " "))
  })

  con.Loop()
}
```
