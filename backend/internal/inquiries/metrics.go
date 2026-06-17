package inquiries

import "sync/atomic"

// connectionEventsTotal counts first-contact connection events since process
// start. Exposed as a simple atomic so the metric can be read by a /metrics
// handler without importing a full metrics library.
var connectionEventsTotal atomic.Int64
