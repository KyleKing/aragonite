// Command outside calls SetTestTransport from a binary that is not a test, which
// must panic rather than quietly reroute a production client.
package main

import "github.com/kyleking/aragonite/transport"

func main() {
	transport.SetTestTransport(nil)()
}
