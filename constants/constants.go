package constants

const (
	Salt         = "dfss-ulak-bibliotheca"
	KeySizeBytes = 32 // SHA-256
	K            = 3
	Alpha        = 3 // Concurrency parameter

	// Proof of Space configuration

	// 2^^16 = 65536 entries, if an attacker wants to attack, it should calculate this many hashes in PoSChallengeTimeout seconds
	// For the production, this value should be increased further, however this might create challenges that non existing on generated plots
	// So, also for production, plot generation number of entries should be increased accordingly
	// Now this nearly creates a file of size ~16MB
	// Probablity of not finding a prefix with T=16 bits in N=400000 entries:
	// ((2 ** 16 - 1) / (2 ** 16)) ** 400000 = 0.2235%

	PosPlotDataDir      = "data/plots" // Directory for storing PoS plots
	PosPrefixBits       = 16           // Number of prefix bits for challenge (T bits)
	PosNumEntries       = 400000       // Number of hash entries to generate in the plot
	PosEntrySize        = 64           // Size of each entry: 32 bytes hash + up to 32 bytes for raw value reference
	PosChallengeTimeout = 5            // Timeout in seconds for PoS challenge response
)
