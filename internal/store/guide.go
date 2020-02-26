package store

type GuideState struct {
	// Used to synchronize engine state and guide.Controller
	Seq int64

	// Set by guide.Controller
	Message string
	Options []string

	// Set by HUD server
	LastClick string
}
