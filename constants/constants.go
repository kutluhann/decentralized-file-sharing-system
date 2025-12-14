package constants

const (
	Salt         = "dfss-ulak-bibliotheca"
	KeySizeBytes = 32 // SHA-256
	K            = 3
	Alpha        = 3 // Concurrency parameter
	
	// Proof of Space configuration
	PlotSize = 50 * 1024 * 1024 // 50 MB
	PlotDataDir = "data/plots"   // Directory for storing PoS plots
)
