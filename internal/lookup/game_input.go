package lookup

import "sync"

// Queue clicks, sim reloads and native meter transfers share one input stream.
// Queue acceptance defers instead of interleaving with an existing operation.
var gameInputMu sync.Mutex
